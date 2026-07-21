import { create } from "zustand";

export type Locale = "en" | "es" | "pt";

type TranslationKeys = {
  // General
  app_name: string;
  no_accounts: string;
  no_accounts_desc: string;
  select_account: string;
  select_account_desc: string;
  // Session
  new_session: string;
  scan_qr: string;
  scan_qr_desc: string;
  linked: string;
  connecting: string;
  logout: string;
  // Calls
  dialer: string;
  call: string;
  calling: string;
  end_call: string;
  active_calls: string;
  no_active_calls: string;
  no_active_calls_desc: string;
  incoming_call: string;
  accept: string;
  reject: string;
  record: string;
  // Devices
  devices: string;
  default_mic: string;
  default_speaker: string;
  // Quality
  quality: string;
  latency: string;
  jitter: string;
  packet_loss: string;
  bitrate: string;
  // History
  history: string;
  call_history: string;
  no_history: string;
  duration: string;
  missed: string;
  // Footer
  accounts: string;
  theme: string;
  language: string;
};

const translations: Record<Locale, TranslationKeys> = {
  en: {
    app_name: "WaCalls",
    no_accounts: "No accounts yet",
    no_accounts_desc: "Create your first WhatsApp account from the sidebar to start calling.",
    select_account: "Select an account",
    select_account_desc: "Choose an account from the sidebar.",
    new_session: "New session",
    scan_qr: "Scan QR code",
    scan_qr_desc: "Open WhatsApp on your phone, go to Linked Devices and scan this code.",
    linked: "Linked",
    connecting: "Connecting…",
    logout: "Log out",
    dialer: "Dialer",
    call: "Call",
    calling: "Calling…",
    end_call: "End call",
    active_calls: "{n} active call(s)",
    no_active_calls: "No active calls",
    no_active_calls_desc: "Dial a number above to start a call.",
    incoming_call: "Incoming call",
    accept: "Accept",
    reject: "Reject",
    record: "Record",
    devices: "Devices",
    default_mic: "Default mic",
    default_speaker: "Default speaker",
    quality: "Quality",
    latency: "Latency",
    jitter: "Jitter",
    packet_loss: "Packet loss",
    bitrate: "Bitrate",
    history: "History",
    call_history: "Call history",
    no_history: "No calls yet",
    duration: "Duration",
    missed: "Missed",
    accounts: "Accounts",
    theme: "Theme",
    language: "Language",
  },
  es: {
    app_name: "WaCalls",
    no_accounts: "Sin cuentas aún",
    no_accounts_desc: "Creá tu primera cuenta de WhatsApp desde la barra lateral para empezar a llamar.",
    select_account: "Elegí una cuenta",
    select_account_desc: "Elegí una cuenta de la barra lateral.",
    new_session: "Nueva sesión",
    scan_qr: "Escaneá el código QR",
    scan_qr_desc: "Abrí WhatsApp en tu celular, andá a Dispositivos vinculados y escaneá este código.",
    linked: "Vinculado",
    connecting: "Conectando…",
    logout: "Cerrar sesión",
    dialer: "Marcador",
    call: "Llamar",
    calling: "Llamando…",
    end_call: "Colgar",
    active_calls: "{n} llamada(s) activa(s)",
    no_active_calls: "Sin llamadas activas",
    no_active_calls_desc: "Marcá un número arriba para iniciar una llamada.",
    incoming_call: "Llamada entrante",
    accept: "Aceptar",
    reject: "Rechazar",
    record: "Grabar",
    devices: "Dispositivos",
    default_mic: "Micrófono predeterminado",
    default_speaker: "Parlante predeterminado",
    quality: "Calidad",
    latency: "Latencia",
    jitter: "Jitter",
    packet_loss: "Pérdida de paquetes",
    bitrate: "Bitrate",
    history: "Historial",
    call_history: "Historial de llamadas",
    no_history: "Sin llamadas aún",
    duration: "Duración",
    missed: "Perdida",
    accounts: "Cuentas",
    theme: "Tema",
    language: "Idioma",
  },
  pt: {
    app_name: "WaCalls",
    no_accounts: "Nenhuma conta ainda",
    no_accounts_desc: "Crie sua primeira conta WhatsApp na barra lateral para começar a ligar.",
    select_account: "Selecione uma conta",
    select_account_desc: "Escolha uma conta na barra lateral.",
    new_session: "Nova sessão",
    scan_qr: "Escaneie o código QR",
    scan_qr_desc: "Abra o WhatsApp no celular, vá para Dispositivos vinculados e escaneie este código.",
    linked: "Vinculado",
    connecting: "Conectando…",
    logout: "Sair",
    dialer: "Discador",
    call: "Ligar",
    calling: "Chamando…",
    end_call: "Encerrar",
    active_calls: "{n} chamada(s) ativa(s)",
    no_active_calls: "Sem chamadas ativas",
    no_active_calls_desc: "Digite um número acima para iniciar uma chamada.",
    incoming_call: "Chamada recebida",
    accept: "Atender",
    reject: "Rejeitar",
    record: "Gravar",
    devices: "Dispositivos",
    default_mic: "Microfone padrão",
    default_speaker: "Alto-falante padrão",
    quality: "Qualidade",
    latency: "Latência",
    jitter: "Jitter",
    packet_loss: "Perda de pacotes",
    bitrate: "Bitrate",
    history: "Histórico",
    call_history: "Histórico de chamadas",
    no_history: "Nenhuma chamada ainda",
    duration: "Duração",
    missed: "Perdida",
    accounts: "Contas",
    theme: "Tema",
    language: "Idioma",
  },
};

type State = {
  locale: Locale;
  t: (key: keyof TranslationValues, params?: Record<string, string | number>) => string;
  setLocale: (l: Locale) => void;
};

type TranslationValues = TranslationKeys;

export const useI18n = create<State>((set, get) => ({
  locale: (localStorage.getItem("wacalls:locale") as Locale) || "en",
  setLocale: (l) => {
    localStorage.setItem("wacalls:locale", l);
    set({ locale: l });
  },
  t: (key, params) => {
    const { locale } = get();
    let str = translations[locale]?.[key] ?? translations.en[key] ?? key;
    if (params) {
      for (const [k, v] of Object.entries(params)) {
        str = str.replace(new RegExp(`\\{${k}\\}`, "g"), String(v));
      }
    }
    return str;
  },
}));
