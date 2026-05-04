package devicelogs

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	defaultDiscoveryRoot = "/dev/serial/by-id"
	defaultScanInterval  = 750 * time.Millisecond
	maxLineBytes         = 1024 * 1024
)

type Kind string

const (
	KindPad    Kind = "pad"
	KindDongle Kind = "dongle"
)

type classification string

const (
	classificationIgnored     classification = "ignored"
	classificationUnknownHPAD classification = "unclassified"
	classificationPad         classification = "pad"
	classificationDongle      classification = "dongle"
	classificationAmbiguous   classification = "ambiguous"
)

type Config struct {
	DiscoveryRoot string
	ScanInterval  time.Duration
	Timestamps    bool
	Debug         bool
	IncludePad    bool
	IncludeDongle bool
	Output        io.Writer
}

type Match struct {
	Matched bool
	Reason  string
}

type Entry struct {
	StablePath   string
	Basename     string
	ResolvedPath string
	ResolveError string
	Pad          Match
	Dongle       Match
	Result       classification
	ResultReason string
}

type snapshot struct {
	root        string
	entries     []Entry
	readError   error
	rootMissing bool
}

type printer struct {
	mu         sync.Mutex
	out        io.Writer
	timestamps bool
}

type roleState struct {
	kind              Kind
	file              *os.File
	current           Entry
	active            bool
	waitingPrinted    bool
	connectedOnce     bool
	connToken         uint64
	lastSelectionNote string
	lastOpenError     string
	padReportSeen     bool
	padKeys           uint8
	padEncoderPressed bool
}

type readerEvent struct {
	kind       Kind
	connToken  uint64
	line       string
	isLine     bool
	disconnect bool
}

type padInputReport struct {
	kind           int
	keys           uint8
	encoderDelta   int
	encoderPressed bool
}

type monitor struct {
	cfg                  Config
	printer              *printer
	states               map[Kind]*roleState
	events               chan readerEvent
	wg                   sync.WaitGroup
	lastDiscoverySummary string
}

func DefaultConfig() Config {
	return Config{
		DiscoveryRoot: defaultDiscoveryRoot,
		ScanInterval:  defaultScanInterval,
		IncludePad:    true,
		IncludeDongle: true,
		Output:        os.Stdout,
	}
}

func Run(ctx context.Context, cfg Config) error {
	resolved, err := normalizeConfig(cfg)
	if err != nil {
		return err
	}

	m := &monitor{
		cfg:     resolved,
		printer: &printer{out: resolved.Output, timestamps: resolved.Timestamps},
		states:  make(map[Kind]*roleState),
		events:  make(chan readerEvent, 256),
	}
	for _, kind := range enabledKinds(resolved) {
		m.states[kind] = &roleState{kind: kind}
	}

	m.printer.plain("HPAD log monitor started")
	m.printer.plain(fmt.Sprintf("scanning %s for HPAD CDC ACM devices", resolved.DiscoveryRoot))
	m.reconcile(ctx)

	ticker := time.NewTicker(resolved.ScanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.shutdown()
			return nil
		case event := <-m.events:
			m.handleEvent(ctx, event)
		case <-ticker.C:
			m.reconcile(ctx)
		}
	}
}

func List(cfg Config) error {
	resolved, err := normalizeConfig(cfg)
	if err != nil {
		return err
	}

	out := resolved.Output
	snap := scanDirectory(resolved.DiscoveryRoot)

	fmt.Fprintf(out, "scanning %s for HPAD CDC ACM devices\n", resolved.DiscoveryRoot)
	switch {
	case snap.rootMissing:
		fmt.Fprintf(out, "discovery root not found: %s\n", resolved.DiscoveryRoot)
		return nil
	case snap.readError != nil:
		return fmt.Errorf("read %s: %w", resolved.DiscoveryRoot, snap.readError)
	}

	if len(snap.entries) == 0 {
		fmt.Fprintf(out, "no serial entries found under %s\n", resolved.DiscoveryRoot)
	} else {
		for _, entry := range snap.entries {
			fmt.Fprintf(out, "\n%s\n", entry.StablePath)
			if entry.ResolvedPath != "" {
				fmt.Fprintf(out, "  resolved: %s\n", entry.ResolvedPath)
			} else {
				fmt.Fprintf(out, "  resolved: unresolved (%s)\n", entry.ResolveError)
			}
			fmt.Fprintf(out, "  pad: %s\n", formatMatch(entry.Pad))
			fmt.Fprintf(out, "  dongle: %s\n", formatMatch(entry.Dongle))
			fmt.Fprintf(out, "  result: %s\n", formatResult(entry))
		}
	}

	for _, kind := range enabledKinds(resolved) {
		entry, ok, note := snap.selectCandidate(kind)
		fmt.Fprintln(out)
		if ok {
			fmt.Fprintf(out, "%s selection: %s -> %s\n", kind, entry.StablePath, entry.ResolvedPath)
		} else {
			fmt.Fprintf(out, "%s selection: none\n", kind)
		}
		if note != "" {
			fmt.Fprintln(out, note)
		}
	}

	return nil
}

func normalizeConfig(cfg Config) (Config, error) {
	if cfg.DiscoveryRoot == "" {
		cfg.DiscoveryRoot = defaultDiscoveryRoot
	}
	if cfg.ScanInterval <= 0 {
		cfg.ScanInterval = defaultScanInterval
	}
	if cfg.Output == nil {
		cfg.Output = os.Stdout
	}
	if !cfg.IncludePad && !cfg.IncludeDongle {
		return Config{}, errors.New("at least one device must be enabled")
	}
	return cfg, nil
}

func enabledKinds(cfg Config) []Kind {
	kinds := make([]Kind, 0, 2)
	if cfg.IncludePad {
		kinds = append(kinds, KindPad)
	}
	if cfg.IncludeDongle {
		kinds = append(kinds, KindDongle)
	}
	return kinds
}

func formatMatch(match Match) string {
	state := "no"
	if match.Matched {
		state = "yes"
	}
	return fmt.Sprintf("%s - %s", state, match.Reason)
}

func formatResult(entry Entry) string {
	switch entry.Result {
	case classificationPad:
		return fmt.Sprintf("pad - %s", entry.ResultReason)
	case classificationDongle:
		return fmt.Sprintf("dongle - %s", entry.ResultReason)
	case classificationAmbiguous:
		return fmt.Sprintf("ambiguous - %s", entry.ResultReason)
	case classificationUnknownHPAD:
		return fmt.Sprintf("unclassified - %s", entry.ResultReason)
	default:
		return fmt.Sprintf("ignored - %s", entry.ResultReason)
	}
}

func scanDirectory(root string) snapshot {
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return snapshot{root: root, rootMissing: true}
		}
		return snapshot{root: root, readError: err}
	}

	results := make([]Entry, 0, len(entries))
	for _, dirEntry := range entries {
		stablePath := filepath.Join(root, dirEntry.Name())
		entry := classifyBasename(stablePath, dirEntry.Name())
		resolved, resolveErr := filepath.EvalSymlinks(stablePath)
		if resolveErr != nil {
			entry.ResolveError = resolveErr.Error()
		} else {
			entry.ResolvedPath = resolved
		}
		results = append(results, entry)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].StablePath < results[j].StablePath
	})

	return snapshot{root: root, entries: results}
}

func classifyBasename(stablePath string, basename string) Entry {
	tokens := tokenizeBasename(basename)
	hasHPAD := hasToken(tokens, "hpad")
	hasPad := hasToken(tokens, "pad")
	hasMacropad := hasToken(tokens, "macropad")
	hasDongle := hasToken(tokens, "dongle")

	entry := Entry{
		StablePath: stablePath,
		Basename:   basename,
	}

	entry.Pad = padMatch(hasHPAD, hasPad, hasMacropad)
	entry.Dongle = dongleMatch(hasHPAD, hasDongle)

	switch {
	case entry.Pad.Matched && entry.Dongle.Matched:
		entry.Result = classificationAmbiguous
		entry.ResultReason = "matched both pad and dongle indicators"
	case entry.Pad.Matched:
		entry.Result = classificationPad
		entry.ResultReason = entry.Pad.Reason
	case entry.Dongle.Matched:
		entry.Result = classificationDongle
		entry.ResultReason = entry.Dongle.Reason
	case hasHPAD:
		entry.Result = classificationUnknownHPAD
		entry.ResultReason = `contains "hpad" but no role token matched (expected "dongle", "macropad", or "pad")`
	default:
		entry.Result = classificationIgnored
		entry.ResultReason = `missing "hpad" token`
	}

	return entry
}

func tokenizeBasename(basename string) []string {
	normalized := strings.ToLower(basename)
	fields := strings.FieldsFunc(normalized, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
	tokens := make([]string, 0, len(fields))
	for _, field := range fields {
		if field == "" {
			continue
		}
		tokens = append(tokens, field)
	}
	return tokens
}

func hasToken(tokens []string, want string) bool {
	for _, token := range tokens {
		if token == want {
			return true
		}
	}
	return false
}

func padMatch(hasHPAD bool, hasPad bool, hasMacropad bool) Match {
	if !hasHPAD {
		return Match{Reason: `missing "hpad" token`}
	}
	if hasMacropad {
		return Match{Matched: true, Reason: `matched "hpad" + "macropad" tokens`}
	}
	if hasPad {
		return Match{Matched: true, Reason: `matched "hpad" + "pad" tokens`}
	}
	return Match{Reason: `missing "macropad" or "pad" token`}
}

func dongleMatch(hasHPAD bool, hasDongle bool) Match {
	if !hasHPAD {
		return Match{Reason: `missing "hpad" token`}
	}
	if hasDongle {
		return Match{Matched: true, Reason: `matched "hpad" + "dongle" tokens`}
	}
	return Match{Reason: `missing "dongle" token`}
}

func (s snapshot) selectCandidate(kind Kind) (Entry, bool, string) {
	want := classificationPad
	if kind == KindDongle {
		want = classificationDongle
	}

	resolved := make([]Entry, 0, 2)
	unresolved := make([]Entry, 0, 1)
	for _, entry := range s.entries {
		if entry.Result != want {
			continue
		}
		if entry.ResolvedPath == "" {
			unresolved = append(unresolved, entry)
			continue
		}
		resolved = append(resolved, entry)
	}

	if len(resolved) == 0 {
		if len(unresolved) > 0 {
			return Entry{}, false, fmt.Sprintf("[%s] matching serial entry exists but its target is unresolved: %s (%s)", kind, unresolved[0].StablePath, unresolved[0].ResolveError)
		}
		if kind == KindPad {
			return s.selectPadFallback()
		}
		return Entry{}, false, ""
	}

	if len(resolved) > 1 {
		ignored := make([]string, 0, len(resolved)-1)
		for _, entry := range resolved[1:] {
			ignored = append(ignored, entry.StablePath)
		}
		return resolved[0], true, fmt.Sprintf("[%s] multiple matching serial entries found; using %s and ignoring %s", kind, resolved[0].StablePath, strings.Join(ignored, ", "))
	}

	return resolved[0], true, ""
}

func (s snapshot) selectPadFallback() (Entry, bool, string) {
	resolved := make([]Entry, 0, 1)
	unresolved := make([]Entry, 0, 1)
	for _, entry := range s.entries {
		if entry.Result != classificationIgnored {
			continue
		}
		if !looksLikeZephyrCDCBackend(entry.Basename) {
			continue
		}
		if entry.ResolvedPath == "" {
			unresolved = append(unresolved, entry)
			continue
		}
		resolved = append(resolved, entry)
	}

	if len(resolved) == 0 {
		if len(unresolved) > 0 {
			return Entry{}, false, fmt.Sprintf("[pad] generic Zephyr CDC ACM candidate exists but its target is unresolved: %s (%s)", unresolved[0].StablePath, unresolved[0].ResolveError)
		}
		return Entry{}, false, ""
	}

	if len(resolved) > 1 {
		choices := make([]string, 0, len(resolved))
		for _, entry := range resolved {
			choices = append(choices, entry.StablePath)
		}
		return Entry{}, false, fmt.Sprintf("[pad] multiple generic Zephyr CDC ACM candidates found; unable to choose between %s", strings.Join(choices, ", "))
	}

	return resolved[0], true, fmt.Sprintf("[pad] using generic Zephyr CDC ACM fallback %s because no explicit HPAD pad identifier was found", resolved[0].StablePath)
}

func looksLikeZephyrCDCBackend(basename string) bool {
	tokens := tokenizeBasename(basename)
	return hasToken(tokens, "zephyr") &&
		hasToken(tokens, "cdc") &&
		hasToken(tokens, "acm") &&
		hasToken(tokens, "backend")
}

func (s snapshot) findByStablePath(stablePath string) (Entry, bool) {
	for _, entry := range s.entries {
		if entry.StablePath == stablePath {
			return entry, true
		}
	}
	return Entry{}, false
}

func (s snapshot) discoveryMessages(debug bool) []string {
	var messages []string
	switch {
	case s.rootMissing:
		messages = append(messages, fmt.Sprintf("discovery root not found: %s", s.root))
	case s.readError != nil:
		messages = append(messages, fmt.Sprintf("discovery error reading %s: %v", s.root, s.readError))
	}

	for _, entry := range s.entries {
		switch entry.Result {
		case classificationAmbiguous:
			messages = append(messages, fmt.Sprintf("discovery: ambiguous HPAD serial entry %s (%s)", entry.StablePath, entry.ResultReason))
		case classificationUnknownHPAD:
			messages = append(messages, fmt.Sprintf("discovery: unclassified HPAD serial entry %s (%s)", entry.StablePath, entry.ResultReason))
		}
	}

	if debug {
		if len(s.entries) == 0 && !s.rootMissing && s.readError == nil {
			messages = append(messages, fmt.Sprintf("debug: no serial entries found under %s", s.root))
		} else if len(s.entries) > 0 {
			parts := make([]string, 0, len(s.entries))
			for _, entry := range s.entries {
				resolved := entry.ResolvedPath
				if resolved == "" {
					resolved = "unresolved"
				}
				parts = append(parts, fmt.Sprintf("%s=%s->%s", entry.Basename, entry.Result, resolved))
			}
			messages = append(messages, fmt.Sprintf("debug: discovered %d serial by-id entries: %s", len(s.entries), strings.Join(parts, ", ")))
		}
	}

	return messages
}

func (m *monitor) reconcile(ctx context.Context) {
	snap := scanDirectory(m.cfg.DiscoveryRoot)
	m.reportDiscoveryChanges(snap)

	if snap.readError == nil && !snap.rootMissing {
		for _, state := range m.states {
			if !state.active || state.file == nil {
				continue
			}
			latest, ok := snap.findByStablePath(state.current.StablePath)
			if !ok || latest.ResolvedPath == "" || latest.ResolvedPath != state.current.ResolvedPath {
				_ = state.file.Close()
			}
		}
	}

	for _, kind := range enabledKinds(m.cfg) {
		state := m.states[kind]
		candidate, ok, note := snap.selectCandidate(kind)
		if note != state.lastSelectionNote {
			if note != "" {
				m.printer.plain(note)
			}
			state.lastSelectionNote = note
		}
		if state.active {
			continue
		}
		if !ok {
			m.printWaiting(state)
			continue
		}
		m.connect(ctx, state, candidate)
	}
}

func (m *monitor) reportDiscoveryChanges(snap snapshot) {
	messages := snap.discoveryMessages(m.cfg.Debug)
	summary := strings.Join(messages, "\n")
	if summary == m.lastDiscoverySummary {
		return
	}
	m.lastDiscoverySummary = summary
	for _, message := range messages {
		m.printer.plain(message)
	}
}

func (m *monitor) connect(ctx context.Context, state *roleState, entry Entry) {
	file, err := os.Open(entry.StablePath)
	if err != nil {
		message := fmt.Sprintf("open failed: %s: %v", entry.StablePath, err)
		if message != state.lastOpenError {
			m.printer.device(state.kind, message)
			state.lastOpenError = message
		}
		m.printWaiting(state)
		return
	}

	state.lastOpenError = ""
	state.active = true
	state.file = file
	state.current = entry
	state.connToken++
	state.waitingPrinted = false
	state.connectedOnce = true

	connToken := state.connToken
	m.printer.device(state.kind, fmt.Sprintf("connected: %s -> %s", entry.StablePath, entry.ResolvedPath))

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		readLoop(ctx, state.kind, connToken, file, m.events)
	}()
}

func readLoop(ctx context.Context, kind Kind, connToken uint64, file *os.File, events chan<- readerEvent) {
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)

	for scanner.Scan() {
		event := readerEvent{kind: kind, connToken: connToken, line: scanner.Text(), isLine: true}
		select {
		case events <- event:
		case <-ctx.Done():
			return
		}
	}

	if ctx.Err() != nil {
		return
	}

	select {
	case events <- readerEvent{kind: kind, connToken: connToken, disconnect: true}:
	case <-ctx.Done():
	}
}

func (m *monitor) handleEvent(ctx context.Context, event readerEvent) {
	state := m.states[event.kind]
	if state == nil || event.connToken != state.connToken {
		return
	}

	if event.isLine {
		if line, ok := state.formatLogLine(event.line); ok {
			m.printer.device(event.kind, line)
		}
		return
	}
	if !event.disconnect || ctx.Err() != nil {
		return
	}

	if state.file != nil {
		_ = state.file.Close()
	}
	state.file = nil
	state.current = Entry{}
	state.active = false
	state.waitingPrinted = false

	m.printer.device(event.kind, "disconnected")
	m.reconcile(ctx)
}

func (s *roleState) formatLogLine(line string) (string, bool) {
	line = strings.TrimRight(line, "\r")
	if strings.TrimSpace(line) == "" {
		return "", false
	}
	if s.kind != KindPad {
		return line, true
	}
	return s.formatPadLogLine(line)
}

func (s *roleState) formatPadLogLine(line string) (string, bool) {
	if report, ok := parsePadInputReportLine(line); ok {
		if report.kind != 0 {
			return line, true
		}
		return s.describePadInput(report)
	}
	if kind, ok := parsePadAckLine(line); ok && kind == 0 {
		return "", false
	}
	return line, true
}

func parsePadInputReportLine(line string) (padInputReport, bool) {
	idx := strings.Index(line, "Queued report kind=")
	if idx < 0 {
		return padInputReport{}, false
	}

	var report padInputReport
	var keys uint32
	var encoderPressed int
	if _, err := fmt.Sscanf(line[idx:], "Queued report kind=%d keys=0x%x enc_delta=%d enc_pressed=%d",
		&report.kind, &keys, &report.encoderDelta, &encoderPressed); err != nil {
		return padInputReport{}, false
	}
	report.keys = uint8(keys)
	report.encoderPressed = encoderPressed != 0
	return report, true
}

func parsePadAckLine(line string) (int, bool) {
	idx := strings.Index(line, "Report delivery acknowledged for kind=")
	if idx < 0 {
		return 0, false
	}

	var kind int
	if _, err := fmt.Sscanf(line[idx:], "Report delivery acknowledged for kind=%d", &kind); err != nil {
		return 0, false
	}
	return kind, true
}

func (s *roleState) describePadInput(report padInputReport) (string, bool) {
	previousKeys := s.padKeys
	previousEncoderPressed := s.padEncoderPressed
	if !s.padReportSeen {
		previousKeys = 0
		previousEncoderPressed = false
	}

	s.padReportSeen = true
	s.padKeys = report.keys
	s.padEncoderPressed = report.encoderPressed

	parts := make([]string, 0, 4)
	if pressed := report.keys &^ previousKeys; pressed != 0 {
		parts = append(parts, formatKeyTransition(pressed, "down"))
	}
	if released := previousKeys &^ report.keys; released != 0 {
		parts = append(parts, formatKeyTransition(released, "up"))
	}
	if report.encoderDelta != 0 {
		parts = append(parts, fmt.Sprintf("encoder %+d", report.encoderDelta))
	}
	switch {
	case !previousEncoderPressed && report.encoderPressed:
		parts = append(parts, "encoder button down")
	case previousEncoderPressed && !report.encoderPressed:
		parts = append(parts, "encoder button up")
	}

	if len(parts) == 0 {
		return "", false
	}
	return "input: " + strings.Join(parts, ", "), true
}

func formatKeyTransition(mask uint8, direction string) string {
	keys := keyNumbers(mask)
	if len(keys) == 1 {
		return fmt.Sprintf("key %s %s", keys[0], direction)
	}
	return fmt.Sprintf("keys %s %s", strings.Join(keys, ","), direction)
}

func keyNumbers(mask uint8) []string {
	keys := make([]string, 0, 6)
	for index := 0; index < 8; index++ {
		if mask&(1<<index) == 0 {
			continue
		}
		keys = append(keys, fmt.Sprintf("%d", index+1))
	}
	return keys
}

func (m *monitor) printWaiting(state *roleState) {
	if state.waitingPrinted {
		return
	}
	if state.connectedOnce {
		m.printer.device(state.kind, "waiting...")
	} else {
		m.printer.plain(fmt.Sprintf("waiting for %s...", state.kind))
	}
	state.waitingPrinted = true
	state.file = nil
	state.active = false
	state.current = Entry{}
}

func (m *monitor) shutdown() {
	for _, state := range m.states {
		if state.file != nil {
			_ = state.file.Close()
			state.file = nil
		}
		state.active = false
	}
	m.wg.Wait()
}

func (p *printer) plain(message string) {
	p.write(message)
}

func (p *printer) device(kind Kind, message string) {
	p.write(deviceLabel(kind) + message)
}

func (p *printer) write(message string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.timestamps {
		fmt.Fprintf(p.out, "%s %s\n", time.Now().Format("15:04:05.000"), message)
		return
	}
	fmt.Fprintln(p.out, message)
}

func deviceLabel(kind Kind) string {
	if kind == KindDongle {
		return "[dongle] "
	}
	return "[pad]    "
}
