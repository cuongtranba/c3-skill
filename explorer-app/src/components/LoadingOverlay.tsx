export function LoadingOverlay({ message }: { message?: string }) {
  return (
    <div className="c3-loading">
      <div className="c3-spinner"></div>
      <div>{message || "Assembling the architecture…"}</div>
    </div>
  );
}
