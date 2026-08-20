"use client";

import { createContext, useContext, useEffect, useState } from "react";
import { pedir } from "./api";
import type { Usuario } from "./tipos";

interface ContextoActor {
  actor: Usuario | null;
  usuarios: Usuario[];
  cambiarActor: (id: string) => void;
  cargando: boolean;
}

const Contexto = createContext<ContextoActor>({
  actor: null,
  usuarios: [],
  cambiarActor: () => {},
  cargando: true,
});

const CLAVE = "vacaciones.actor";

export function ProveedorActor({ children }: { children: React.ReactNode }) {
  const [usuarios, setUsuarios] = useState<Usuario[]>([]);
  const [actor, setActor] = useState<Usuario | null>(null);
  const [cargando, setCargando] = useState(true);

  useEffect(() => {
    pedir<Usuario[]>("/api/usuarios", null)
      .then((lista) => {
        setUsuarios(lista);
        const guardado = localStorage.getItem(CLAVE);
        const elegido = lista.find((u) => u.id === guardado) ?? lista[0] ?? null;
        setActor(elegido);
      })
      .catch(() => setUsuarios([]))
      .finally(() => setCargando(false));
  }, []);

  function cambiarActor(id: string) {
    const elegido = usuarios.find((u) => u.id === id);
    if (!elegido) return;
    localStorage.setItem(CLAVE, id);
    setActor(elegido);
  }

  return (
    <Contexto.Provider value={{ actor, usuarios, cambiarActor, cargando }}>
      {children}
    </Contexto.Provider>
  );
}

export function useActor() {
  return useContext(Contexto);
}
