"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { pedir } from "@/lib/api";
import { useActor } from "@/lib/actor";
import { Tarjeta, fecha } from "@/components/ui";
import { TablaMovimientos } from "@/components/TablaMovimientos";
import { FormularioOtorgamiento } from "@/components/FormularioOtorgamiento";
import type { Finiquito, Movimiento, Saldo } from "@/lib/tipos";

export default function DetalleColaborador() {
  const { actor, cargando } = useActor();
  const params = useParams<{ id: string }>();
  const id = params.id;

  const [saldo, setSaldo] = useState<Saldo | null>(null);
  const [movimientos, setMovimientos] = useState<Movimiento[]>([]);
  const [finiquito, setFiniquito] = useState<Finiquito | null>(null);
  const [corte, setCorte] = useState("");

  const recargar = useCallback(() => {
    if (!actor || actor.rol !== "RRHH") return;
    pedir<Saldo>(`/api/colaboradores/${id}/saldo`, actor.id).then(setSaldo);
    pedir<Finiquito>(`/api/colaboradores/${id}/finiquito`, actor.id).then(setFiniquito);
    const sufijo = corte ? `?hasta=${corte}` : "";
    pedir<Movimiento[]>(`/api/colaboradores/${id}/movimientos${sufijo}`, actor.id).then(
      setMovimientos,
    );
  }, [actor, id, corte]);

  useEffect(recargar, [recargar]);

  if (cargando) return <p className="text-slate-500">Cargando…</p>;
  if (actor?.rol !== "RRHH") {
    return <p className="text-slate-500">Esta vista requiere rol de RRHH.</p>;
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">{saldo?.colaborador_nombre ?? "…"}</h1>
        <p className="text-sm text-slate-500">
          Ingreso {saldo ? fecha(saldo.fecha_ingreso) : "—"}
        </p>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {(saldo?.por_tipo ?? []).map((t) => (
          <Tarjeta key={t.tipo_id} titulo={t.tipo_nombre || t.tipo_codigo}>
            <p className="font-mono text-3xl">{t.disponible}</p>
            <p className="mt-1 text-xs text-slate-500">disponibles</p>
          </Tarjeta>
        ))}

        {saldo?.devengado_no_otorgado && (
          <Tarjeta titulo="Devengado no otorgado">
            <p className="font-mono text-3xl">{saldo.devengado_no_otorgado}</p>
            <p className="mt-1 text-xs text-slate-500">
              del período en curso · solo visible para RRHH
            </p>
          </Tarjeta>
        )}
      </div>

      {finiquito && (
        <Tarjeta titulo="Si se desvincula hoy">
          <div className="grid gap-4 text-sm sm:grid-cols-3">
            <div>
              <p className="text-slate-500">Proporcional del período</p>
              <p className="font-mono text-2xl">{finiquito.Proporcional.Dias}</p>
              <p className="mt-1 text-xs text-slate-500">
                {finiquito.Proporcional.MesesCompletos} meses × 1,25 + (
                {finiquito.Proporcional.DiasRestantes}/30) × 1,25
              </p>
            </div>
            <div>
              <p className="text-slate-500">Disponible pagable</p>
              <p className="font-mono text-2xl">{finiquito.DisponiblePagable}</p>
              <p className="mt-1 text-xs text-slate-500">
                solo tipos marcados como pagables en finiquito
              </p>
            </div>
            <div>
              <p className="text-slate-500">Total a pagar</p>
              <p className="font-mono text-2xl font-semibold">{finiquito.Total}</p>
              <p className="mt-1 text-xs text-slate-500">días hábiles</p>
            </div>
          </div>
        </Tarjeta>
      )}

      <Tarjeta titulo="Otorgar saldo especial">
        <FormularioOtorgamiento colaboradorId={id} alOtorgar={recargar} />
      </Tarjeta>

      <Tarjeta titulo="Historial de movimientos">
        <label className="mb-4 flex items-center gap-2 text-sm">
          <span className="text-slate-600">Reconstruir el saldo a la fecha</span>
          <input
            type="date"
            value={corte}
            onChange={(e) => setCorte(e.target.value)}
            className="rounded border border-slate-300 px-2 py-1"
          />
          {corte && (
            <button
              onClick={() => setCorte("")}
              className="text-slate-500 underline hover:text-slate-900"
            >
              ver todo
            </button>
          )}
        </label>
        <TablaMovimientos movimientos={movimientos} />
      </Tarjeta>
    </div>
  );
}
