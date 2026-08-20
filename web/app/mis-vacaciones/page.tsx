"use client";

import { useCallback, useEffect, useState } from "react";
import { pedir } from "@/lib/api";
import { useActor } from "@/lib/actor";
import { Tarjeta, Etiqueta, fecha } from "@/components/ui";
import { FormularioSolicitud } from "@/components/FormularioSolicitud";
import { TablaMovimientos } from "@/components/TablaMovimientos";
import type { Movimiento, Saldo, Solicitud } from "@/lib/tipos";

export default function MisVacaciones() {
  const { actor, cargando } = useActor();
  const [saldo, setSaldo] = useState<Saldo | null>(null);
  const [solicitudes, setSolicitudes] = useState<Solicitud[]>([]);
  const [movimientos, setMovimientos] = useState<Movimiento[]>([]);

  const recargar = useCallback(() => {
    if (!actor) return;
    pedir<Saldo>(`/api/colaboradores/${actor.id}/saldo`, actor.id).then(setSaldo);
    pedir<Solicitud[]>("/api/solicitudes", actor.id).then(setSolicitudes);
    pedir<Movimiento[]>(`/api/colaboradores/${actor.id}/movimientos`, actor.id).then(
      setMovimientos,
    );
  }, [actor]);

  useEffect(recargar, [recargar]);

  if (cargando || !actor) return <p className="text-slate-500">Cargando…</p>;

  const tipos = saldo?.por_tipo ?? [];
  const porVencer = tipos.flatMap((t) =>
    (t.por_vencer ?? []).map((b) => ({ ...b, tipo: t.tipo_nombre || t.tipo_codigo })),
  );

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">Hola, {actor.nombre}</h1>
        <p className="text-sm text-slate-500">
          Tienes <strong className="font-mono">{saldo?.total_disponible ?? "—"}</strong> días
          disponibles en total.
        </p>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {tipos.map((t) => (
          <Tarjeta key={t.tipo_id} titulo={t.tipo_nombre || t.tipo_codigo}>
            <p className="font-mono text-3xl">{t.disponible}</p>
            <p className="mt-1 text-xs text-slate-500">días disponibles</p>
            {t.pendiente !== "0" && (
              <p className="mt-2 text-xs text-amber-700">
                {t.pendiente} comprometidos en solicitudes pendientes ·{" "}
                <strong>{t.solicitable}</strong> solicitables
              </p>
            )}
          </Tarjeta>
        ))}
      </div>

      {porVencer.length > 0 && (
        <Tarjeta titulo="Próximos a vencer">
          <ul className="space-y-1 text-sm">
            {porVencer.map((b) => (
              <li key={b.OtorgamientoID} className="flex justify-between">
                <span className="text-slate-600">{b.tipo}</span>
                <span>
                  <strong className="font-mono">{b.Dias}</strong> días vencen el{" "}
                  <span className="font-mono">{fecha(b.VenceEl)}</span>
                </span>
              </li>
            ))}
          </ul>
        </Tarjeta>
      )}

      <Tarjeta titulo="Solicitar vacaciones">
        {tipos.length > 0 ? (
          <FormularioSolicitud tipos={tipos} alCrear={recargar} />
        ) : (
          <p className="text-sm text-slate-500">No tienes saldo disponible para solicitar.</p>
        )}
      </Tarjeta>

      <Tarjeta titulo="Mis solicitudes">
        {solicitudes.length === 0 ? (
          <p className="text-sm text-slate-500">Sin solicitudes.</p>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-slate-200 text-left text-xs uppercase tracking-wide text-slate-500">
                <th className="py-2 pr-4">Desde</th>
                <th className="py-2 pr-4">Hasta</th>
                <th className="py-2 pr-4">Tipo</th>
                <th className="py-2 pr-4 text-right">Días hábiles</th>
                <th className="py-2">Estado</th>
              </tr>
            </thead>
            <tbody>
              {solicitudes.map((s) => (
                <tr key={s.ID} className="border-b border-slate-100">
                  <td className="py-2 pr-4 font-mono text-xs">{fecha(s.Desde)}</td>
                  <td className="py-2 pr-4 font-mono text-xs">{fecha(s.Hasta)}</td>
                  <td className="py-2 pr-4 text-slate-600">{s.TipoCodigo}</td>
                  <td className="py-2 pr-4 text-right font-mono">{s.DiasHabiles}</td>
                  <td className="py-2">
                    <Etiqueta valor={s.Estado} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Tarjeta>

      <Tarjeta titulo="Mi historial">
        <TablaMovimientos movimientos={movimientos} />
      </Tarjeta>
    </div>
  );
}
