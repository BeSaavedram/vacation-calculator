import { Etiqueta, fecha } from "./ui";
import type { Movimiento } from "@/lib/tipos";

export function TablaMovimientos({ movimientos }: { movimientos: Movimiento[] }) {
  if (movimientos.length === 0) {
    return <p className="text-sm text-slate-500">Sin movimientos.</p>;
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-slate-200 text-left text-xs uppercase tracking-wide text-slate-500">
            <th className="py-2 pr-4">Fecha</th>
            <th className="py-2 pr-4">Clase</th>
            <th className="py-2 pr-4">Tipo</th>
            <th className="py-2 pr-4 text-right">Días</th>
            <th className="py-2 pr-4">Motivo</th>
            <th className="py-2">Actor</th>
          </tr>
        </thead>
        <tbody>
          {movimientos.map((m) => {
            const negativo = m.Cantidad.startsWith("-");
            return (
              <tr key={m.ID} className="border-b border-slate-100 align-top">
                <td className="py-2 pr-4 whitespace-nowrap font-mono text-xs">
                  {fecha(m.FechaEfectiva)}
                </td>
                <td className="py-2 pr-4">
                  <Etiqueta valor={m.Clase} />
                </td>
                <td className="py-2 pr-4 whitespace-nowrap text-slate-600">{m.TipoCodigo}</td>
                <td
                  className={`py-2 pr-4 text-right font-mono ${
                    negativo ? "text-rose-700" : "text-emerald-700"
                  }`}
                >
                  {negativo ? m.Cantidad : `+${m.Cantidad}`}
                </td>
                <td className="py-2 pr-4 text-slate-600">{m.Motivo}</td>
                <td className="py-2 whitespace-nowrap text-slate-500">{m.ActorNombre}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
