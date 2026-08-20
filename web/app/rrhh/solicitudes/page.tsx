"use client";

import { useCallback, useEffect, useState } from "react";
import { pedir, postear } from "@/lib/api";
import { useActor } from "@/lib/actor";
import { Tarjeta, Etiqueta, Aviso, fecha } from "@/components/ui";
import type { Solicitud } from "@/lib/tipos";

export default function Solicitudes() {
  const { actor, cargando } = useActor();
  const [solicitudes, setSolicitudes] = useState<Solicitud[]>([]);
  const [error, setError] = useState("");

  const recargar = useCallback(() => {
    if (!actor || actor.rol !== "RRHH") return;
    pedir<Solicitud[]>("/api/solicitudes", actor.id).then(setSolicitudes);
  }, [actor]);

  useEffect(recargar, [recargar]);

  async function decidir(id: string, accion: "aprobar" | "rechazar") {
    if (!actor) return;
    setError("");
    try {
      await postear(`/api/solicitudes/${id}/${accion}`, actor.id);
      recargar();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error desconocido");
    }
  }

  if (cargando) return <p className="text-slate-500">Cargando…</p>;
  if (actor?.rol !== "RRHH") {
    return <p className="text-slate-500">Esta vista requiere rol de RRHH.</p>;
  }

  const pendientes = solicitudes.filter((s) => s.Estado === "PENDIENTE");
  const decididas = solicitudes.filter((s) => s.Estado !== "PENDIENTE");

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Solicitudes</h1>
      {error && <Aviso mensaje={error} />}

      <Tarjeta titulo={`Pendientes (${pendientes.length})`}>
        {pendientes.length === 0 ? (
          <p className="text-sm text-slate-500">No hay solicitudes pendientes.</p>
        ) : (
          <ul className="divide-y divide-slate-100">
            {pendientes.map((s) => (
              <li key={s.ID} className="flex items-center justify-between py-3">
                <div className="text-sm">
                  <p className="font-medium">{s.ColaboradorNom}</p>
                  <p className="text-slate-500">
                    <span className="font-mono">{fecha(s.Desde)}</span> al{" "}
                    <span className="font-mono">{fecha(s.Hasta)}</span> ·{" "}
                    <strong className="font-mono">{s.DiasHabiles}</strong> días hábiles ·{" "}
                    {s.TipoCodigo}
                  </p>
                </div>
                <div className="flex gap-2">
                  <button
                    onClick={() => decidir(s.ID, "aprobar")}
                    className="rounded bg-emerald-600 px-3 py-1.5 text-sm font-medium text-white"
                  >
                    Aprobar
                  </button>
                  <button
                    onClick={() => decidir(s.ID, "rechazar")}
                    className="rounded border border-slate-300 px-3 py-1.5 text-sm"
                  >
                    Rechazar
                  </button>
                </div>
              </li>
            ))}
          </ul>
        )}
        <p className="mt-4 text-xs text-slate-500">
          Al aprobar, el consumo se reparte entre las bolsas del tipo empezando por la que
          vence antes, y cada tramo genera su propio movimiento en el historial.
        </p>
      </Tarjeta>

      <Tarjeta titulo="Historial de solicitudes">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-slate-200 text-left text-xs uppercase tracking-wide text-slate-500">
              <th className="py-2 pr-4">Colaborador</th>
              <th className="py-2 pr-4">Desde</th>
              <th className="py-2 pr-4">Hasta</th>
              <th className="py-2 pr-4 text-right">Días</th>
              <th className="py-2">Estado</th>
            </tr>
          </thead>
          <tbody>
            {decididas.map((s) => (
              <tr key={s.ID} className="border-b border-slate-100">
                <td className="py-2 pr-4">{s.ColaboradorNom}</td>
                <td className="py-2 pr-4 font-mono text-xs">{fecha(s.Desde)}</td>
                <td className="py-2 pr-4 font-mono text-xs">{fecha(s.Hasta)}</td>
                <td className="py-2 pr-4 text-right font-mono">{s.DiasHabiles}</td>
                <td className="py-2">
                  <Etiqueta valor={s.Estado} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </Tarjeta>
    </div>
  );
}
