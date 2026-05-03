type StatusChipProps = {
  title: string;
  status: string;
  tone: string;
};

type StatusCardProps = {
  title: string;
  status: string;
  detail: string;
  tone: string;
};

export function StatusChip(props: StatusChipProps) {
  return (
    <div className={`status-chip ${toneClass(props.tone)}`}>
      <span className="status-chip-title">{props.title}</span>
      <strong>{props.status}</strong>
    </div>
  );
}

export function StatusCard(props: StatusCardProps) {
  return (
    <article className={`status-card ${toneClass(props.tone)}`}>
      <div className="status-card-topline">
        <span>{props.title}</span>
        <strong>{props.status}</strong>
      </div>
      <p>{props.detail}</p>
    </article>
  );
}

function toneClass(tone: string) {
  switch (tone) {
    case "connected":
    case "usb_power":
      return "tone-connected";
    case "disconnected":
    case "waiting":
      return "tone-waiting";
    case "not_detected":
      return "tone-alert";
    default:
      return "tone-neutral";
  }
}