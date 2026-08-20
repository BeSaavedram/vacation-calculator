import type { Metadata } from "next";
import "./globals.css";
import { ProveedorActor } from "@/lib/actor";
import { SelectorUsuario } from "@/components/SelectorUsuario";

export const metadata: Metadata = {
  title: "Gestión de Vacaciones",
  description: "Saldos de vacaciones derivados de un ledger inmutable",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="es">
      <body className="min-h-screen bg-slate-50 text-slate-900 antialiased">
        <ProveedorActor>
          <SelectorUsuario />
          <main className="mx-auto max-w-6xl px-6 py-8">{children}</main>
        </ProveedorActor>
      </body>
    </html>
  );
}
