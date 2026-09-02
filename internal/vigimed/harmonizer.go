package vigimed

import (
	"strings"
	"unicode"
)

// normalizeString strips accents, lowers case, and trims whitespace.
func normalizeString(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	// Replace common Portuguese accents
	replacer := strings.NewReplacer(
		"á", "a", "à", "a", "ã", "a", "â", "a",
		"é", "e", "ê", "e",
		"í", "i",
		"ó", "o", "õ", "o", "ô", "o",
		"ú", "u", "ü", "u",
		"ç", "c",
	)
	s = replacer.Replace(s)
	// Filter non-alphanumeric except spaces and hyphens
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == ' ' {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

var knownDrugs = []DrugMapping{
	{
		CanonicalName: "Semaglutide",
		DCBName:       "Semaglutida",
		ATCCode:       "A10BJ06",
		Aliases:       []string{"semaglutide", "semaglutida", "ozempic", "wegovy", "rybelsus"},
	},
	{
		CanonicalName: "Metformin",
		DCBName:       "Metformina",
		ATCCode:       "A10BA02",
		Aliases:       []string{"metformin", "metformina", "glifage", "glucomet", "dimefor", "gliphage"},
	},
	{
		CanonicalName: "Dapagliflozin",
		DCBName:       "Dapagliflozina",
		ATCCode:       "A10BK01",
		Aliases:       []string{"dapagliflozin", "dapagliflozina", "forxiga", "xigduo"},
	},
	{
		CanonicalName: "Dipyrone",
		DCBName:       "Dipirona",
		ATCCode:       "N02BB02",
		Aliases:       []string{"dipyrone", "dipirona", "metamizole", "metamizol", "novalgina", "anador", "lisador", "dorflex"},
	},
	{
		CanonicalName: "Rosuvastatin",
		DCBName:       "Rosuvastatina",
		ATCCode:       "C10AA07",
		Aliases:       []string{"rosuvastatin", "rosuvastatina", "crestor", "rosumax", "vivacor", "plenance"},
	},
	{
		CanonicalName: "Pembrolizumab",
		DCBName:       "Pembrolizumabe",
		ATCCode:       "L01FF02",
		Aliases:       []string{"pembrolizumab", "pembrolizumabe", "keytruda"},
	},
	{
		CanonicalName: "Adalimumab",
		DCBName:       "Adalimumabe",
		ATCCode:       "L04AB04",
		Aliases:       []string{"adalimumab", "adalimumabe", "humira", "amgevita", "hyrimoz", "hulio"},
	},
	{
		CanonicalName: "Omeprazole",
		DCBName:       "Omeprazol",
		ATCCode:       "A02BC01",
		Aliases:       []string{"omeprazole", "omeprazol", "losec", "peprazol", "gastrium"},
	},
	{
		CanonicalName: "Losartan",
		DCBName:       "Losartana",
		ATCCode:       "C09CA01",
		Aliases:       []string{"losartan", "losartana", "losartana potassica", "cozaar", "corus", "aradois"},
	},
	{
		CanonicalName: "Amoxicillin",
		DCBName:       "Amoxicilina",
		ATCCode:       "J01CA04",
		Aliases:       []string{"amoxicillin", "amoxicilina", "amoxil", "clavulin", "novocilin"},
	},
	{
		CanonicalName: "Ibuprofen",
		DCBName:       "Ibuprofeno",
		ATCCode:       "M01AE01",
		Aliases:       []string{"ibuprofen", "ibuprofeno", "advil", "alivium", "buscofem"},
	},
	{
		CanonicalName: "Paracetamol",
		DCBName:       "Paracetamol",
		ATCCode:       "N02BE01",
		Aliases:       []string{"paracetamol", "acetaminophen", "tylenol", "parador", "dôrico"},
	},
}

var reactionCrosswalk = map[string]string{
	// PT-BR -> EN
	"nausea":                     "NAUSEA",
	"vomito":                     "VOMITING",
	"diarreia":                   "DIARRHOEA",
	"dor abdominal":              "ABDOMINAL PAIN",
	"pancreatite":                "PANCREATITIS",
	"hipoglicemia":               "HYPOGLYCAEMIA",
	"cefaleia":                   "HEADACHE",
	"tontura":                    "DIZZINESS",
	"fadiga":                     "FATIGUE",
	"astenia":                    "ASTHENIA",
	"dispneia":                   "DYSPNOEA",
	"injuria renal aguda":        "ACUTE KIDNEY INJURY",
	"acidose lactica":            "LACTIC ACIDOSIS",
	"constipacao":                "CONSTIPATION",
	"perda de peso":              "WEIGHT DECREASED",
	"ganho de peso":              "WEIGHT INCREASED",
	"apetite diminuido":          "DECREASED APPETITE",
	"artralgia":                  "ARTHRALGIA",
	"mialgia":                    "MYALGIA",
	"rabdomiolise":               "RHABDOMYOLISIS",
	"erupcao cutanea":            "RASH",
	"prurido":                    "PRURITUS",
	"choque anafilatico":         "ANAPHYLACTIC SHOCK",
	"agranulocitose":             "AGRANULOCYTOSIS",
	"leucopenia":                 "LEUKOPENIA",
	"febre":                      "PYREXIA",
	"insuficiencia hepatica":     "HEPATIC FAILURE",
	"reacao adversa a farmaco":   "ADVERSE DRUG REACTION",
	"uso fora da bula":          "OFF LABEL USE",
	"erro de medicacao":          "MEDICATION ERROR",
	"produto falsificado":        "COUNTERFEIT PRODUCT ADMINISTERED",
	"tosse":                      "COUGH",
	"queda":                      "FALL",
}

// ResolveDrug finds the matching DrugMapping for a given search query.
func ResolveDrug(query string) (*DrugMapping, bool) {
	norm := normalizeString(query)
	if norm == "" {
		return nil, false
	}

	for _, d := range knownDrugs {
		for _, alias := range d.Aliases {
			if norm == alias || strings.Contains(norm, alias) || strings.Contains(alias, norm) {
				return &d, true
			}
		}
	}

	// Fallback for generic capitalization
	return &DrugMapping{
		CanonicalName: strings.Title(strings.ToLower(query)),
		DCBName:       strings.Title(strings.ToLower(query)),
		ATCCode:       "NOT_INDEXED",
		Aliases:       []string{norm},
	}, false
}

// TranslatePTBRtoEN returns the English MedDRA PT for a Portuguese term.
func TranslatePTBRtoEN(term string) string {
	norm := normalizeString(term)
	if en, ok := reactionCrosswalk[norm]; ok {
		return en
	}
	return strings.ToUpper(term)
}
