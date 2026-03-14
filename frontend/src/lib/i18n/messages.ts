import type { LocaleCode } from "@/lib/i18n/config";

export type Messages = {
  localeName: string;
  appDescription: string;
  backToList: string;
  latestVotations: string;
  loadingVotationError: string;
  loadingVotationsError: string;
  votationNotFound: string;
  noVotations: string;
  titleUnavailable: string;
  translationInProgress: string;
  dateUnknown: string;
  levelUnknown: string;
  statusUnknown: string;
  levelFederal: string;
  levelCantonal: string;
  levelCommunal: string;
  statusPast: string;
  statusUpcoming: string;
  qaTitle: string;
  qaIntro: string;
  qaDefaultQuestion: string;
  qaAutoLoading: string;
  qaAutoLoadError: string;
  qaSubmitError: string;
  qaNoAnswerYet: string;
  qaQuestionLabel: string;
  qaQuestionEmpty: string;
  qaQuestionTooLong: string;
  qaQuestionPlaceholder: string;
  qaSend: string;
  qaSending: string;
  qaQuestionPrefix: string;
  locationLabel: string;
  locationFederal: string;
  locationUnknown: string;
  cantonLabel: string;
  communeLabel: string;
};

const MESSAGES: Record<LocaleCode, Messages> = {
  fr: {
    localeName: "Français",
    appDescription: "PoC de visualisation des votations suisses",
    backToList: "Retour à la liste",
    latestVotations: "Dernières votations enregistrées (plus récentes en haut).",
    loadingVotationError: "Impossible de charger cette votation.",
    loadingVotationsError: "Impossible de charger les votations pour le moment.",
    votationNotFound: "Votation introuvable.",
    noVotations: "Aucune votation disponible.",
    titleUnavailable: "Titre indisponible",
    translationInProgress: "traduction en cours",
    dateUnknown: "Date inconnue",
    levelUnknown: "Niveau inconnu",
    statusUnknown: "Statut inconnu",
    levelFederal: "Fédéral",
    levelCantonal: "Cantonal",
    levelCommunal: "Communal",
    statusPast: "Passée",
    statusUpcoming: "À venir",
    qaTitle: "Questions et réponses",
    qaIntro: "Une première réponse est chargée automatiquement, puis vous pouvez poser des questions supplémentaires.",
    qaDefaultQuestion: "Quels sont les enjeux principaux de cette votation ?",
    qaAutoLoading: "Génération du résumé initial en cours...",
    qaAutoLoadError: "Impossible de charger le résumé automatique.",
    qaSubmitError: "Impossible de récupérer une réponse pour le moment.",
    qaNoAnswerYet: "Aucune réponse pour le moment.",
    qaQuestionLabel: "Poser une question supplémentaire",
    qaQuestionEmpty: "La question ne peut pas être vide.",
    qaQuestionTooLong: "La question ne peut pas dépasser {max} caractères.",
    qaQuestionPlaceholder: "Ex. : Quels sont les impacts économiques à court terme ?",
    qaSend: "Envoyer",
    qaSending: "Envoi...",
    qaQuestionPrefix: "Question",
    locationLabel: "Localisation",
    locationFederal: "Suisse",
    locationUnknown: "Localisation inconnue",
    cantonLabel: "Canton",
    communeLabel: "Commune",
  },
  de: {
    localeName: "Deutsch",
    appDescription: "PoC zur Visualisierung von Schweizer Abstimmungen",
    backToList: "Zuruck zur Liste",
    latestVotations: "Neueste Abstimmungen (neueste zuerst).",
    loadingVotationError: "Diese Abstimmung konnte nicht geladen werden.",
    loadingVotationsError: "Die Abstimmungen konnen derzeit nicht geladen werden.",
    votationNotFound: "Abstimmung nicht gefunden.",
    noVotations: "Keine Abstimmungen verfugbar.",
    titleUnavailable: "Titel nicht verfugbar",
    translationInProgress: "Ubersetzung lauft",
    dateUnknown: "Unbekanntes Datum",
    levelUnknown: "Unbekannte Ebene",
    statusUnknown: "Unbekannter Status",
    levelFederal: "Bund",
    levelCantonal: "Kanton",
    levelCommunal: "Gemeinde",
    statusPast: "Vergangen",
    statusUpcoming: "Bevorstehend",
    qaTitle: "Fragen und Antworten",
    qaIntro: "Eine erste Antwort wird automatisch geladen, danach konnen Sie weitere Fragen stellen.",
    qaDefaultQuestion: "Was sind die wichtigsten Auswirkungen dieser Abstimmung?",
    qaAutoLoading: "Die erste Zusammenfassung wird erstellt...",
    qaAutoLoadError: "Die automatische Zusammenfassung konnte nicht geladen werden.",
    qaSubmitError: "Derzeit kann keine Antwort abgerufen werden.",
    qaNoAnswerYet: "Noch keine Antwort vorhanden.",
    qaQuestionLabel: "Eine weitere Frage stellen",
    qaQuestionEmpty: "Die Frage darf nicht leer sein.",
    qaQuestionTooLong: "Die Frage darf {max} Zeichen nicht uberschreiten.",
    qaQuestionPlaceholder: "Beispiel: Welche kurzfristigen wirtschaftlichen Folgen gibt es?",
    qaSend: "Senden",
    qaSending: "Senden...",
    qaQuestionPrefix: "Frage",
    locationLabel: "Ort",
    locationFederal: "Schweiz",
    locationUnknown: "Unbekannter Ort",
    cantonLabel: "Kanton",
    communeLabel: "Gemeinde",
  },
  it: {
    localeName: "Italiano",
    appDescription: "PoC per visualizzare le votazioni svizzere",
    backToList: "Torna all'elenco",
    latestVotations: "Ultime votazioni registrate (piu recenti in alto).",
    loadingVotationError: "Impossibile caricare questa votazione.",
    loadingVotationsError: "Impossibile caricare le votazioni al momento.",
    votationNotFound: "Votazione non trovata.",
    noVotations: "Nessuna votazione disponibile.",
    titleUnavailable: "Titolo non disponibile",
    translationInProgress: "traduzione in corso",
    dateUnknown: "Data sconosciuta",
    levelUnknown: "Livello sconosciuto",
    statusUnknown: "Stato sconosciuto",
    levelFederal: "Federale",
    levelCantonal: "Cantonale",
    levelCommunal: "Comunale",
    statusPast: "Passata",
    statusUpcoming: "Imminente",
    qaTitle: "Domande e risposte",
    qaIntro: "Una prima risposta viene caricata automaticamente, poi puoi porre altre domande.",
    qaDefaultQuestion: "Quali sono gli impatti principali di questa votazione?",
    qaAutoLoading: "Generazione del riepilogo iniziale in corso...",
    qaAutoLoadError: "Impossibile caricare il riepilogo automatico.",
    qaSubmitError: "Impossibile recuperare una risposta al momento.",
    qaNoAnswerYet: "Nessuna risposta al momento.",
    qaQuestionLabel: "Fai una domanda aggiuntiva",
    qaQuestionEmpty: "La domanda non puo essere vuota.",
    qaQuestionTooLong: "La domanda non puo superare {max} caratteri.",
    qaQuestionPlaceholder: "Es.: Quali sono gli impatti economici a breve termine?",
    qaSend: "Invia",
    qaSending: "Invio...",
    qaQuestionPrefix: "Domanda",
    locationLabel: "Localizzazione",
    locationFederal: "Svizzera",
    locationUnknown: "Localizzazione sconosciuta",
    cantonLabel: "Cantone",
    communeLabel: "Comune",
  },
  rm: {
    localeName: "Rumantsch",
    appDescription: "PoC per visualizar votaziuns svizras",
    backToList: "Anavos a la glista",
    latestVotations: "Ultimas votaziuns registradas (las pli novas sisum).",
    loadingVotationError: "Impussibel da chargiar questa votaziun.",
    loadingVotationsError: "Impussibel da chargiar las votaziuns per il mument.",
    votationNotFound: "Votaziun betg chattada.",
    noVotations: "Naginas votaziuns disponiblas.",
    titleUnavailable: "Titel betg disponibel",
    translationInProgress: "translaziun en curs",
    dateUnknown: "Data nunenconuschenta",
    levelUnknown: "Nivel nunenconuschent",
    statusUnknown: "Status nunenconuschent",
    levelFederal: "Federal",
    levelCantonal: "Chantunal",
    levelCommunal: "Communal",
    statusPast: "Passada",
    statusUpcoming: "Imminenta",
    qaTitle: "Dumondas e novitads",
    qaIntro: "Ina emprima resposta vegn chargiada automaticamain, suenter pos ti far ulteriuras dumondas.",
    qaDefaultQuestion: "Tge èn ils effects principals da questa votaziun?",
    qaAutoLoading: "Generaziun dal resumaziun iniziala en curs...",
    qaAutoLoadError: "Impussibel da chargiar il resumaziun automatic.",
    qaSubmitError: "Impussibel da survegnir ina resposta per il mument.",
    qaNoAnswerYet: "Anc nagina resposta.",
    qaQuestionLabel: "Far ina dumonda supplementara",
    qaQuestionEmpty: "La dumonda na dastga betg esser vida.",
    qaQuestionTooLong: "La dumonda na dastga betg surpassar {max} segns.",
    qaQuestionPlaceholder: "Ex.: Tge èn ils effects economics a curta vista?",
    qaSend: "Trametter",
    qaSending: "Trametter...",
    qaQuestionPrefix: "Dumonda",
    locationLabel: "Localisaziun",
    locationFederal: "Svizra",
    locationUnknown: "Localisaziun nunenconuschenta",
    cantonLabel: "Chantun",
    communeLabel: "Vischnanca",
  },
  en: {
    localeName: "English",
    appDescription: "PoC to visualize Swiss votes",
    backToList: "Back to list",
    latestVotations: "Latest votes recorded (most recent first).",
    loadingVotationError: "Unable to load this vote.",
    loadingVotationsError: "Unable to load votes right now.",
    votationNotFound: "Vote not found.",
    noVotations: "No votes available.",
    titleUnavailable: "Title unavailable",
    translationInProgress: "translation in progress",
    dateUnknown: "Unknown date",
    levelUnknown: "Unknown level",
    statusUnknown: "Unknown status",
    levelFederal: "Federal",
    levelCantonal: "Cantonal",
    levelCommunal: "Communal",
    statusPast: "Past",
    statusUpcoming: "Upcoming",
    qaTitle: "Questions and answers",
    qaIntro: "An initial answer is loaded automatically, then you can ask additional questions.",
    qaDefaultQuestion: "What are the main impacts of this vote?",
    qaAutoLoading: "Generating initial summary...",
    qaAutoLoadError: "Unable to load the automatic summary.",
    qaSubmitError: "Unable to fetch an answer right now.",
    qaNoAnswerYet: "No answer yet.",
    qaQuestionLabel: "Ask another question",
    qaQuestionEmpty: "Question cannot be empty.",
    qaQuestionTooLong: "Question cannot exceed {max} characters.",
    qaQuestionPlaceholder: "Example: What are the short-term economic impacts?",
    qaSend: "Send",
    qaSending: "Sending...",
    qaQuestionPrefix: "Question",
    locationLabel: "Location",
    locationFederal: "Switzerland",
    locationUnknown: "Unknown location",
    cantonLabel: "Canton",
    communeLabel: "Municipality",
  },
};

export function getMessages(locale: LocaleCode): Messages {
  return MESSAGES[locale];
}
