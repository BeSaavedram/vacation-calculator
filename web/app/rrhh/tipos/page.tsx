"use client";

import { useCallback, useEffect, useState } from "react";
import { pedir, postear } from "@/lib/api";
import { useActor } from "@/lib/actor";
import { Tarjeta, Aviso } from "@/components/ui";
import type { Parametros, TipoDeVacacion } from "@/lib/tipos";

const NUEVO_POR_DEFECTO = {
  codigo: "RENDIMIENTO",
  nombre: "Días por rendimiento",
  politica_devengo: "manual",
  politica_vencimiento: "dias_fijos",
  prioridad_consumo: 5,
  unidad_habil: true,
  pagable_en_finiquito: false,
  parametros: { dias_fijos: 180 } as Parametros,
};

export default function Tipos() {
  const { actor, cargando } = useActor();
  const [tipos, setTipos] = useState<TipoDeVacacion[]>([]);
  const [nuevo, setNuevo] = useState(NUEVO_POR_DEFECTO);
  const [error, setError] = useState("");
  const [ok, setOk] = useState("");

  const recargar = useCallback(() => {
    if (!actor) return;
    pedir<TipoDeVacacion[]>("/api/tipos-vacacion", actor.id).then(setTipos);
  }, [actor]);

  useEffect(recargar, [recargar]);

  async function crear(e: React.FormEvent) {
    e.preventDefault();
    if (!actor) return;

    setError("");
    setOk("");
    try {
      await postear("/api/tipos-vacacion", actor.id, nuevo);
      setOk(`Tipo "${nuevo.nombre}" creado. Ya se puede otorgar, sin desplegar código.`);
      recargar();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error desconocido");
    }
  }

  if (cargando) return <p className="text-slate-500">Cargando…</p>;
  if (actor?.rol !== "RRHH") {
    return <p className="text-slate-500">Esta vista requiere rol de RRHH.</p>;
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">Tipos de vacación</h1>
        <p className="text-sm text-slate-500">
          Cada tipo compone tres políticas intercambiables. Agregar uno nuevo es crear un
          registro, no escribir código.
        </p>
      </div>

      <Tarjeta titulo="Configurados">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-slate-200 text-left text-xs uppercase tracking-wide text-slate-500">
              <th className="py-2 pr-4">Código</th>
              <th className="py-2 pr-4">Devengo</th>
              <th className="py-2 pr-4">Vencimiento</th>
              <th className="py-2 pr-4 text-right">Prioridad</th>
              <th className="py-2 pr-4">Pagable</th>
              <th className="py-2">Parámetros</th>
            </tr>
          </thead>
          <tbody>
            {tipos.map((t) => (
              <tr key={t.ID} className="border-b border-slate-100">
                <td className="py-2 pr-4 font-medium">{t.Codigo}</td>
                <td className="py-2 pr-4 font-mono text-xs">{t.PoliticaDevengo}</td>
                <td className="py-2 pr-4 font-mono text-xs">{t.PoliticaVencimiento}</td>
                <td className="py-2 pr-4 text-right">{t.PrioridadConsumo}</td>
                <td className="py-2 pr-4">{t.PagableEnFiniquito ? "sí" : "no"}</td>
                <td className="py-2 font-mono text-xs text-slate-500">
                  {JSON.stringify(t.Parametros)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </Tarjeta>

      <Tarjeta titulo="Crear un tipo nuevo">
        <form onSubmit={crear} className="space-y-4">
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <label className="block text-sm">
              <span className="mb-1 block text-slate-600">Código</span>
              <input
                value={nuevo.codigo}
                onChange={(e) => setNuevo({ ...nuevo, codigo: e.target.value })}
                className="w-full rounded border border-slate-300 px-2 py-1.5 font-mono"
              />
            </label>

            <label className="block text-sm">
              <span className="mb-1 block text-slate-600">Nombre</span>
              <input
                value={nuevo.nombre}
                onChange={(e) => setNuevo({ ...nuevo, nombre: e.target.value })}
                className="w-full rounded border border-slate-300 px-2 py-1.5"
              />
            </label>

            <label className="block text-sm">
              <span className="mb-1 block text-slate-600">Devengo</span>
              <select
                value={nuevo.politica_devengo}
                onChange={(e) => setNuevo({ ...nuevo, politica_devengo: e.target.value })}
                className="w-full rounded border border-slate-300 px-2 py-1.5"
              >
                <option value="manual">manual</option>
                <option value="aniversario_legal">aniversario_legal</option>
                <option value="anio_calendario">anio_calendario</option>
              </select>
            </label>

            <label className="block text-sm">
              <span className="mb-1 block text-slate-600">Vencimiento</span>
              <select
                value={nuevo.politica_vencimiento}
                onChange={(e) =>
                  setNuevo({ ...nuevo, politica_vencimiento: e.target.value })
                }
                className="w-full rounded border border-slate-300 px-2 py-1.5"
              >
                <option value="dias_fijos">dias_fijos</option>
                <option value="fin_de_anio">fin_de_anio</option>
                <option value="n_periodos">n_periodos</option>
                <option value="no_vence">no_vence</option>
              </select>
            </label>
          </div>

          <label className="block text-sm">
            <span className="mb-1 block text-slate-600">
              Parámetros (JSON: dias_base, dias_fijos, n_periodos…)
            </span>
            <input
              value={JSON.stringify(nuevo.parametros)}
              onChange={(e) => {
                try {
                  setNuevo({ ...nuevo, parametros: JSON.parse(e.target.value) });
                  setError("");
                } catch {
                  setError("Los parámetros deben ser JSON válido");
                }
              }}
              className="w-full rounded border border-slate-300 px-2 py-1.5 font-mono text-xs"
            />
          </label>

          {error && <Aviso mensaje={error} />}
          {ok && <Aviso mensaje={ok} tono="ok" />}

          <button
            type="submit"
            className="rounded bg-slate-900 px-4 py-2 text-sm font-medium text-white"
          >
            Crear tipo
          </button>
        </form>
      </Tarjeta>
    </div>
  );
}
