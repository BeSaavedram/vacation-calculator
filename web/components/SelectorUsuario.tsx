"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useActor } from "@/lib/actor";

export function SelectorUsuario() {
  const { actor, usuarios, cambiarActor } = useActor();
  const ruta = usePathname();

  const enlaces = actor?.rol === "RRHH"
    ? [
        { href: "/rrhh", texto: "Colaboradores" },
        { href: "/rrhh/solicitudes", texto: "Solicitudes" },
        { href: "/rrhh/tipos", texto: "Tipos de vacación" },
      ]
    : [{ href: "/mis-vacaciones", texto: "Mis vacaciones" }];

  return (
    <header className="border-b border-slate-200 bg-white">
      <div className="mx-auto flex max-w-6xl items-center justify-between gap-6 px-6 py-3">
        <div className="flex items-center gap-6">
          <span className="font-semibold">Gestión de Vacaciones</span>
          <nav className="flex gap-4 text-sm">
            {enlaces.map((e) => (
              <Link
                key={e.href}
                href={e.href}
                className={
                  ruta === e.href
                    ? "font-medium text-slate-900"
                    : "text-slate-500 hover:text-slate-900"
                }
              >
                {e.texto}
              </Link>
            ))}
          </nav>
        </div>

        <label className="flex items-center gap-2 text-sm">
          <span className="text-slate-500">Ver como</span>
          <select
            value={actor?.id ?? ""}
            onChange={(e) => cambiarActor(e.target.value)}
            className="rounded border border-slate-300 bg-white px-2 py-1"
          >
            {usuarios.map((u) => (
              <option key={u.id} value={u.id}>
                {u.nombre} · {u.rol === "RRHH" ? "RRHH" : "Colaborador"}
              </option>
            ))}
          </select>
        </label>
      </div>
    </header>
  );
}
