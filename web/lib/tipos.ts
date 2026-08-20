// Los campos decimales son string a propósito: el API los serializa así para
// que nunca pasen por el float64 de JavaScript. Se muestran, no se calculan.

export type Rol = "COLABORADOR" | "RRHH";

export interface Usuario {
  id: string;
  nombre: string;
  rol: Rol;
  email: string;
}

export interface BolsaPorVencer {
  OtorgamientoID: string;
  Dias: string;
  VenceEl: string;
}

export interface SaldoDeTipo {
  tipo_id: string;
  tipo_codigo: string;
  tipo_nombre: string;
  disponible: string;
  pendiente: string;
  solicitable: string;
  por_vencer: BolsaPorVencer[] | null;
}

export interface Proporcional {
  PeriodoDesde: string;
  Hasta: string;
  MesesCompletos: number;
  DiasRestantes: number;
  Dias: string;
}

export interface Saldo {
  colaborador_id: string;
  colaborador_nombre: string;
  fecha_ingreso: string;
  al_dia: string;
  total_disponible: string;
  por_tipo: SaldoDeTipo[] | null;
  devengado_no_otorgado?: string;
  proporcional?: Proporcional;
}

export interface Movimiento {
  ID: string;
  OtorgamientoID: string;
  SolicitudID: string | null;
  Cantidad: string;
  Clase: string;
  FechaEfectiva: string;
  FechaRegistro: string;
  Motivo: string;
  ClaveIdempotencia: string;
  TipoCodigo: string;
  TipoNombre: string;
  ActorNombre: string;
  VenceEl: string | null;
}

export interface Solicitud {
  ID: string;
  ColaboradorID: string;
  ColaboradorNom: string;
  TipoID: string;
  TipoCodigo: string;
  Desde: string;
  Hasta: string;
  DiasHabiles: string;
  Estado: "PENDIENTE" | "APROBADA" | "RECHAZADA" | "CANCELADA";
  CreadaEl: string;
}

export interface ColaboradorConSaldo {
  id: string;
  nombre: string;
  email: string;
  rol: Rol;
  fecha_ingreso: string;
  anios_antiguedad: number;
  disponible: string;
}

export interface Parametros {
  dias_base?: string;
  progresivo_activo?: boolean;
  progresivo_umbral_meses?: number;
  progresivo_antiguedad_minima_meses?: number;
  progresivo_cadencia_anios?: number;
  progresivo_dias_por_tramo?: string;
  n_periodos?: number;
  dias_fijos?: number;
}

export interface TipoDeVacacion {
  ID: string;
  Codigo: string;
  Nombre: string;
  PoliticaDevengo: string;
  PoliticaVencimiento: string;
  Parametros: Parametros;
  PrioridadConsumo: number;
  UnidadHabil: boolean;
  PagableEnFiniquito: boolean;
}

export interface Finiquito {
  Proporcional: Proporcional;
  DisponiblePagable: string;
  Total: string;
}

export interface Preview {
  desde: string;
  hasta: string;
  dias_habiles: string;
  dias_corridos: number;
}
