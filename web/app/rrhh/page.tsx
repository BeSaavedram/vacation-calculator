"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { pedir } from "@/lib/api";
import { useActor } from "@/lib/actor";
import { Tarjeta, fecha } from "@/components/ui";
import type { ColaboradorConSaldo } from "@/lib/tipos";

export default function Colaboradores() {
  const { actor, cargando } = useActor();
  const [filas, setFilas] = useState<ColaboradorConSaldo[]>([]);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!actor) return;
    pedir<ColaboradorConSaldo[]>("/api/colaboradores", actor.id)
      .then(setFilas)
      .catch((e) => setError(e.message));
  }, [actor]);

  if (cargando) return <p className="text-slate-500">Cargando…</p>;
  if (actor?.rol !== "RRHH") {
    return <p className="text-slate-500">Esta vista requiere rol de RRHH.</p>;
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Colaboradores</h1>
      {error && <p className="text-sm text-rose-700">{error}</p>}

      <Tarjeta>
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-slate-200 text-left text-xs uppercase tracking-wide text-slate-500">
              <th className="py-2 pr-4">Nombre</th>
              <th className="py-2 pr-4">Ingreso</th>
              <th className="py-2 pr-4 text-right">Antigüedad</th>
              <th className="py-2 pr-4 text-right">Disponible</th>
              <th className="py-2"></th>
            </tr>
          </thead>
          <tbody>
            {filas.map((c) => (
              <tr key={c.id} className="border-b border-slate-100">
                <td className="py-2 pr-4">
                  <span className="font-medium">{c.nombre}</span>
                  <span className="ml-2 text-xs text-slate-400">{c.email}</span>
                </td>
                <td className="py-2 pr-4 font-mono text-xs">{fecha(c.fecha_ingreso)}</td>
                <td className="py-2 pr-4 text-right">{c.anios_antiguedad} años</td>
                <td className="py-2 pr-4 text-right font-mono">{c.disponible}</td>
                <td className="py-2 text-right">
                  <Link
                    href={`/rrhh/colaboradores/${c.id}`}
                    className="text-sm text-slate-600 underline hover:text-slate-900"
                  >
                    Ver detalle
                  </Link>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </Tarjeta>
    </div>
  );
}
