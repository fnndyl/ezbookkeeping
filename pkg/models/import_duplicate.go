package models

// ImportDuplicateCandidate deliberately excludes account and category.
type ImportDuplicateCandidate struct {
	Type      TransactionType `json:"type" binding:"required,min=1,max=4"`
	Time      int64           `json:"time" binding:"required,min=1,max=253402300799"`
	UtcOffset int16           `json:"utcOffset" binding:"min=-840,max=840"`
	Amount    int64           `json:"amount" binding:"validTransactionAmount"`
	Comment   string          `json:"comment"`
}

type ImportDuplicateCheckRequest struct {
	Transactions []*ImportDuplicateCandidate `json:"transactions" binding:"required,max=65535,dive,required"`
}

type ImportDuplicateMatch struct {
	Duplicate  bool `json:"duplicate"`
	Occurrence int  `json:"occurrence"`
}
