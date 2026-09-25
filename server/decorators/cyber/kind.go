package cyber

type Kind string

const (
	KindCVE    Kind = "cve"
	KindCWE    Kind = "cwe"
	KindAttack Kind = "attack"
	KindIP     Kind = "ip"
	KindHash   Kind = "hash"
)

var Kinds = []Kind{KindCVE, KindCWE, KindAttack, KindIP, KindHash}

var kindLabels = map[Kind]string{
	KindCVE:    "Vulnerability",
	KindCWE:    "Weakness",
	KindAttack: "ATT&CK",
	KindIP:     "IP address",
	KindHash:   "File hash",
}

func (k Kind) Label() string { return kindLabels[k] }

func (k Kind) Known() bool {
	_, ok := kindLabels[k]
	return ok
}
