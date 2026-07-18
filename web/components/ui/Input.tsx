import { type InputHTMLAttributes } from "react";

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  mono?: boolean;
  label?: string;
}

export function Input({ mono = false, label, id, className = "", ...props }: InputProps) {
  const input = (
    <input
      id={id}
      className={`w-full rounded-sm border border-border-default bg-surface-default px-3 py-2 text-sm text-content-default placeholder:text-content-muted outline-none focus:border-border-accent ${
        mono ? "font-mono" : "font-sans"
      } ${className}`}
      {...props}
    />
  );

  if (!label) return input;

  return (
    <label htmlFor={id} className="flex flex-col gap-1.5">
      <span className="text-xs font-medium text-content-secondary">{label}</span>
      {input}
    </label>
  );
}
