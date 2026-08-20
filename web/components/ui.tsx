export function Tarjeta({
  titulo,
  children,
}: {
  titulo?: string;
  children: React.ReactNode;
}) {
  return (
    <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
      {titulo && (
        <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-slate-500">
          {titulo}
        </h2>
      )}
      {children}
    </section>
  );
}

export function Etiqueta({ valor }: { valor: string }) {
  const colores: Record<string, string> = {
    PENDIENTE: "bg-amber-100 text-amber-800",
    APROBADA: "bg-emerald-100 text-emerald-800",
    RECHAZADA: "bg-rose-100 text-rose-800",
    CANCELADA: "bg-slate-100 text-slate-600",
    ACCRUAL: "bg-emerald-100 text-emerald-800",
    CONSUMPTION: "bg-sky-100 text-sky-800",
    EXPIRATION: "bg-rose-100 text-rose-800",
    ADJUSTMENT: "bg-violet-100 text-violet-800",
    REVERSAL: "bg-orange-100 text-orange-800",
    OPENING_BALANCE: "bg-slate-100 text-slate-700",
    SETTLEMENT_PAYOUT: "bg-indigo-100 text-indigo-800",
  };
  return (
    <span
      className={`inline-block rounded px-2 py-0.5 text-xs font-medium ${
        colores[valor] ?? "bg-slate-100 text-slate-700"
      }`}
    >
      {valor}
    </span>
  );
}

export function Aviso({ mensaje, tono = "error" }: { mensaje: string; tono?: "error" | "ok" }) {
  const clases =
    tono === "ok"
      ? "border-emerald-200 bg-emerald-50 text-emerald-800"
      : "border-rose-200 bg-rose-50 text-rose-800";
  return <p className={`rounded border px-3 py-2 text-sm ${clases}`}>{mensaje}</p>;
}

// fecha formatea un ISO del API como YYYY-MM-DD, sin zona horaria.
export function fecha(iso: string): string {
  return iso.slice(0, 10);
}
