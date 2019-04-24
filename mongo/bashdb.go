package mongo

// Databse 1, store the basic transaction metadata
type Transac struct {
	Tx_BlockHash string
	Tx_BlockNum string 
	Tx_FromAddr string
	Tx_Gas string
	Tx_GasPrice string
	Tx_Hash string 
	Tx_Input string 
	Tx_Nonce string
	Tx_R string
 	Tx_S string
	Tx_ToAddr string
	Tx_Index string
	Tx_V string
	Tx_Value string
}

// Database 2, store the basic transaction metadata
type Trace struct {
	Tx_Hash string
	Tx_Trace string
}

// Database 3: receipt
type Rece struct{
	// BlockHash
	// BlockNumber
	Re_contractAddress string
	Re_CumulativeGasUsed string
	// from
	Re_GasUsed string
	Re_Status  string
	// to
	Re_TxHash string
	// TransactionIndex
	// Store the pre-execution error
	Re_FailReason string
}

var BashNum int = 50
var BashTxs = make([]interface{}, BashNum)
var BashTrs = make([]interface{}, BashNum)
var BashRes = make([]interface{}, BashNum)
var CurrentNum int = 0
