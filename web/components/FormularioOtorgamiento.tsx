"use client";

import { useEffect, useState } from "react";
import { pedir, postear } from "@/lib/api";
import { useActor } from "@/lib/actor";
import { Aviso } from "./ui";
import type { TipoDeVacacion } from "@/lib/tipos";

export function FormularioOtorgamiento({
  colaboradorId,
  alOtorgar,
}: {
  colaboradorId: string;
  alOtorgar: () => void;
}) {
  const { actor } = useActor();
  const [tipos, setTipos] = useState<TipoDeVacacion[]>([]);
  const [tipoId, setTipoId] = useState("");
  const [dias, setDias] = useState("");
  const [motivo, setMotivo] = useState("");
  const [error, setError] = useState("");
  const [ok, setOk] = useState("");

  useEffect(() => {
    if (!actor) return;
    pedir<TipoDeVacacion[]>("/api/tipos-vacacion", actor.id).then((lista) => {
      setTipos(lista);
      setTipoId((actual) => actual || lista[0]?.ID || "");
    });
  }, [actor]);

  async function enviar(e: React.FormEvent) {
    e.preventDefault();
    if (!actor) return;

    setError("");
    setOk("");
    try {
      await postear("/api/otorgamientos", actor.id, {
        colaborador_id: colaboradorId,
        tipo_id: tipoId,
        dias,
        motivo,
      });
      setOk("Días otorgados. Quedó registrado en el historial con tu nombre.");
      setDias("");
      setMotivo("");
      alOtorgar();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error desconocido");
    }
  }

  return (
    <form onSubmit={enviar} className="space-y-3">
      <div className="grid gap-3 sm:grid-cols-3">
        <label className="block text-sm">
          <span className="mb-1 block text-slate-600">Tipo</span>
          <select
            value={tipoId}
            onChange={(e) => setTipoId(e.target.value)}
            className="w-full rounded border border-slate-300 px-2 py-1.5"
          >
            {tipos.map((t) => (
              <option key={t.ID} value={t.ID}>
                {t.Nombre}
              </option>
            ))}
          </select>
        </label>

        <label className="block text-sm">
          <span className="mb-1 block text-slate-600">Días</span>
          <input
            value={dias}
            onChange={(e) => setDias(e.target.value)}
            placeholder="2.5"
            required
            className="w-full rounded border border-slate-300 px-2 py-1.5 font-mono"
          />
        </label>

        <label className="block text-sm sm:col-span-1">
          <span className="mb-1 block text-slate-600">Motivo (obligatorio)</span>
          <input
            value={motivo}
            onChange={(e) => setMotivo(e.target.value)}
            placeholder="Reconocimiento por proyecto Q3"
            required
            className="w-full rounded border border-slate-300 px-2 py-1.5"
          />
        </label>
      </div>

      {error && <Aviso mensaje={error} />}
      {ok && <Aviso mensaje={ok} tono="ok" />}

      <button
        type="submit"
        className="rounded bg-slate-900 px-4 py-2 text-sm font-medium text-white"
      >
        Otorgar días
      </button>
    </form>
  );
}
