"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useActor } from "@/lib/actor";

export default function Inicio() {
  const { actor, cargando } = useActor();
  const router = useRouter();

  useEffect(() => {
    if (cargando || !actor) return;
    router.replace(actor.rol === "RRHH" ? "/rrhh" : "/mis-vacaciones");
  }, [actor, cargando, router]);

  if (cargando) return <p className="text-slate-500">Cargando…</p>;
  if (!actor) {
    return (
      <p className="text-slate-500">
        No hay usuarios. ¿Corriste <code>make seed</code> y está el API arriba?
      </p>
    );
  }
  return <p className="text-slate-500">Redirigiendo…</p>;
}
