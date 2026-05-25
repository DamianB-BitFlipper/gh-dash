export function DiffUrlForm({
  className,
  initialUrl,
  onUrlChange,
}: {
  className?: string;
  initialUrl: string;
  onUrlChange?: (url: string) => void;
  placeholder?: string;
  inputClassName?: string;
}) {
  return (
    <input
      className={className}
      value={initialUrl}
      readOnly
      aria-label="Diff URL"
      onChange={(event) => onUrlChange?.(event.currentTarget.value)}
    />
  );
}
