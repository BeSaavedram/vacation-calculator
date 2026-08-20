const BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export class ErrorApi extends Error {}

// pedir hace la llamada agregando el header de actor. Todo el frontend pasa por
// acá: no hay otra forma de hablar con el API.
export async function pedir<T>(
  ruta: string,
  actorId: string | null,
  init?: RequestInit,
): Promise<T> {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (actorId) headers["X-Actor-Id"] = actorId;

  const respuesta = await fetch(`${BASE}${ruta}`, { ...init, headers });

  if (!respuesta.ok) {
    let mensaje = `${respuesta.status} ${respuesta.statusText}`;
    try {
      const cuerpo = await respuesta.json();
      if (cuerpo?.error) mensaje = cuerpo.error;
    } catch {
      // La respuesta no era JSON; nos quedamos con el status.
    }
    throw new ErrorApi(mensaje);
  }

  if (respuesta.status === 204) return undefined as T;
  return respuesta.json() as Promise<T>;
}

export function postear<T>(ruta: string, actorId: string | null, cuerpo?: unknown) {
  return pedir<T>(ruta, actorId, {
    method: "POST",
    body: cuerpo === undefined ? undefined : JSON.stringify(cuerpo),
  });
}
