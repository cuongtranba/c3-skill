export function LiveBanner({ issues }: { issues: string[] }) {
  if (!issues.length) return null;
  return (
    <div className="c3-live-banner">
      <b>Model update rejected — showing last valid state.</b>
      <div className="c3-live-banner-issues">
        {issues.slice(0, 5).map((it, i) => (
          <div key={i}>{it}</div>
        ))}
        {issues.length > 5 && <div>… {issues.length - 5} more issue(s)</div>}
      </div>
    </div>
  );
}
