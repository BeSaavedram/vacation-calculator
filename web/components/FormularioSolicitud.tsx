"use client";

import { useEffect, useState } from "react";
import { pedir, postear } from "@/lib/api";
import { useActor } from "@/lib/actor";
import { Aviso } from "./ui";
import type { Preview, SaldoDeTipo } from "@/lib/tipos";

export function FormularioSolicitud({
  tipos,
  alCrear,
}: {
  tipos: SaldoDeTipo[];
  alCrear: () => void;
}) {
  const { actor } = useActor();
  const [tipoId, setTipoId] = useState(tipos[0]?.tipo_id ?? "");
  const [desde, setDesde] = useState("");
  const [hasta, setHasta] = useState("");
  const [preview, setPreview] = useState<Preview | null>(null);
  const [error, setError] = useState("");
  const [ok, setOk] = useState("");
  const [enviando, setEnviando] = useState(false);

  // El preview lo calcula el API, no el navegador: los días hábiles dependen
  // del calendario de feriados, que vive en la base.
  useEffect(() => {
    if (!desde || !hasta || hasta < desde || !actor) {
      setPreview(null);
      return;
    }
    pedir<Preview>(`/api/solicitudes/preview?desde=${desde}&hasta=${hasta}`, actor.id)
      .then(setPreview)
      .catch(() => setPreview(null));
  }, [desde, hasta, actor]);

  async function enviar(e: React.FormEvent) {
    e.preventDefault();
    if (!actor) return;

    setEnviando(true);
    setError("");
    setOk("");
    try {
      await postear("/api/solicitudes", actor.id, { tipo_id: tipoId, desde, hasta });
      setOk("Solicitud creada. Queda pendiente de aprobación.");
      setDesde("");
      setHasta("");
      setPreview(null);
      alCrear();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error desconocido");
    } finally {
      setEnviando(false);
    }
  }

  const seleccionado = tipos.find((t) => t.tipo_id === tipoId);

  return (
    <form onSubmit={enviar} className="space-y-4">
      <div className="grid gap-4 sm:grid-cols-3">
        <label className="block text-sm">
          <span className="mb-1 block text-slate-600">Tipo</span>
          <select
            value={tipoId}
            onChange={(e) => setTipoId(e.target.value)}
            className="w-full rounded border border-slate-300 px-2 py-1.5"
          >
            {tipos.map((t) => (
              <option key={t.tipo_id} value={t.tipo_id}>
                {t.tipo_nombre || t.tipo_codigo}
              </option>
            ))}
          </select>
        </label>

        <label className="block text-sm">
          <span className="mb-1 block text-slate-600">Desde</span>
          <input
            type="date"
            value={desde}
            onChange={(e) => setDesde(e.target.value)}
            required
            className="w-full rounded border border-slate-300 px-2 py-1.5"
          />
        </label>

        <label className="block text-sm">
          <span className="mb-1 block text-slate-600">Hasta</span>
          <input
            type="date"
            value={hasta}
            onChange={(e) => setHasta(e.target.value)}
            required
            className="w-full rounded border border-slate-300 px-2 py-1.5"
          />
        </label>
      </div>

      {preview && (
        <p className="text-sm text-slate-600">
          Descuenta{" "}
          <strong className="font-mono">{preview.dias_habiles}</strong> días hábiles
          de {preview.dias_corridos} corridos.
          {seleccionado && (
            <>
              {" "}Tienes <strong className="font-mono">{seleccionado.solicitable}</strong>{" "}
              solicitables.
            </>
          )}
        </p>
      )}

      {error && <Aviso mensaje={error} />}
      {ok && <Aviso mensaje={ok} tono="ok" />}

      <button
        type="submit"
        disabled={enviando || !preview}
        className="rounded bg-slate-900 px-4 py-2 text-sm font-medium text-white disabled:opacity-40"
      >
        {enviando ? "Enviando…" : "Solicitar"}
      </button>
    </form>
  );
}
