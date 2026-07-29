import Link from "next/link";
import { type ReactNode } from "react";

export function PageHeader({
  eyebrow,
  title,
  description,
  action,
}: {
  eyebrow?: string;
  title: string;
  description?: string;
  action?: { label: string; href: string } | ReactNode;
}) {
  return (
    <header className="page-header">
      <div>
        {eyebrow ? <p className="eyebrow">{eyebrow}</p> : null}
        <h1>{title}</h1>
        {description ? <p>{description}</p> : null}
      </div>
      {action && typeof action === "object" && "href" in action ? (
        <Link className="button button-primary button-md" href={action.href}>{action.label}</Link>
      ) : action}
    </header>
  );
}
