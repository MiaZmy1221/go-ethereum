// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package core

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/misc"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"

	// Add
	"fmt"
	"github.com/ethereum/go-ethereum/common/hexutil"
	// "gopkg.in/mgo.v2/bson"
	"github.com/ethereum/go-ethereum/mongo"
	// "time"
	"gopkg.in/mgo.v2"
	"encoding/json"
)



// StateProcessor is a basic Processor, which takes care of transitioning
// state from one point to another.
//
// StateProcessor implements Processor.
type StateProcessor struct {
	config *params.ChainConfig // Chain configuration options
	bc     *BlockChain         // Canonical block chain
	engine consensus.Engine    // Consensus engine used for block rewards
}

// NewStateProcessor initialises a new StateProcessor.
func NewStateProcessor(config *params.ChainConfig, bc *BlockChain, engine consensus.Engine) *StateProcessor {
	return &StateProcessor{
		config: config,
		bc:     bc,
		engine: engine,
	}
}

// Process processes the state changes according to the Ethereum rules by running
// the transaction messages using the statedb and applying any rewards to both
// the processor (coinbase) and any included uncles.
//
// Process returns the receipts and logs accumulated during the process and
// returns the amount of gas that was used in the process. If any of the
// transactions failed to execute due to insufficient gas it will return an error.
func (p *StateProcessor) Process(block *types.Block, statedb *state.StateDB, cfg vm.Config) (types.Receipts, []*types.Log, uint64, error) {
	// print("at the beginning of the process\n")
	// start_tempt1 := time.Now()

	var (
		receipts types.Receipts
		usedGas  = new(uint64)
		header   = block.Header()
		allLogs  []*types.Log
		gp       = new(GasPool).AddGas(block.GasLimit())
	)
	// Mutate the block and state according to any hard-fork specs
	if p.config.DAOForkSupport && p.config.DAOForkBlock != nil && p.config.DAOForkBlock.Cmp(block.Number()) == 0 {
		misc.ApplyDAOHardFork(statedb)
	}

	// print("before process all the transactions , time is ", fmt.Sprintf("%s", time.Since(start_tempt1)) , "\n")

	// Iterate over and process the individual transactions
	for i, tx := range block.Transactions() {
		// print("transaction tx ", tx.Hash().Hex(), "\n")
		// start_tempt2 := time.Now()

		statedb.Prepare(tx.Hash(), block.Hash(), i)
		receipt, _, err := ApplyTransaction(p.config, p.bc, nil, gp, statedb, header, tx, usedGas, cfg)
		if err != nil {
			return nil, nil, 0, err
		}
		receipts = append(receipts, receipt)
		allLogs = append(allLogs, receipt.Logs...)

		// print("apply the transactions time is ", fmt.Sprintf("%s", time.Since(start_tempt2)) , "\n")

	}
	// Finalize the block, applying any consensus engine specific extras (e.g. block rewards)
	p.engine.Finalize(p.bc, header, statedb, block.Transactions(), block.Uncles(), receipts)

	return receipts, allLogs, *usedGas, nil
}


// ApplyTransaction attempts to apply a transaction to the given state database
// and uses the input parameters for its environment. It returns the receipt
// for the transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
func ApplyTransaction(config *params.ChainConfig, bc ChainContext, author *common.Address, gp *GasPool, statedb *state.StateDB, header *types.Header, tx *types.Transaction, usedGas *uint64, cfg vm.Config) (*types.Receipt, uint64, error) {
	// print("at the beginning of the applytransaction\n")
	// start_tempt1 := time.Now()

	// mongo.CurrentTx = tx.Hash().Hex() 
	mongo.TraceGlobal.Reset()
	mongo.TxVMErr = ""

	msg, err := tx.AsMessage(types.MakeSigner(config, header.Number))
	if err != nil {
		// print("applytransaction stage1.1 return time is ", fmt.Sprintf("%s", time.Since(start_tempt1)) , "\n")
		return nil, 0, err
	}

	// print("applytransaction stage1.1 normal time is ", fmt.Sprintf("%s", time.Since(start_tempt1)) , "\n")
	// start_tempt11 := time.Now()

	// Create a new context to be used in the EVM environment
	context := NewEVMContext(msg, header, bc, author)

	// print("applytransaction stage1.2 time is ", fmt.Sprintf("%s", time.Since(start_tempt11)) , "\n")
	// start_tempt12 := time.Now()

	toaddr := ""
	if msg.To() == nil {
		toaddr = "0x0"
	} else {
		tempt := *msg.To()
		toaddr = tempt.String()
	}

	// write transaction to the array
	mongo.CurrentBlockNum = header.Number.Uint64()
	mongo.BashTxs[mongo.CurrentNum] = mongo.Transac{statedb.BlockHash().Hex(), header.Number.String(), 
					msg.From().String(), fmt.Sprintf("%d", tx.Gas()), tx.GasPrice().String(), 
					tx.Hash().Hex(), hexutil.Encode(tx.Data()), fmt.Sprintf("0x%x", tx.Nonce()), 
					fmt.Sprintf("0x%x", tx.R()), fmt.Sprintf("0x%x", tx.S()), toaddr, 
					fmt.Sprintf("0x%x", statedb.TxIndex()), fmt.Sprintf("0x%x", tx.V()), msg.Value().String()}

	// print("applytransaction stage1.3 time is ", fmt.Sprintf("%s", time.Since(start_tempt12)) , "\n")
	// start_tempt13 := time.Now()

	// Create a new environment which holds all relevant information
	// about the transaction and calling mechanisms.
	// vmenv := vm.NewEVM(context, statedb, config, cfg)
	vmenv := vm.NewEVMWithFlag(context, statedb, config, cfg, false)

	// print("applytransaction stage1.4 time is ", fmt.Sprintf("%s", time.Since(start_tempt13)) , "\n")
	// start_tempt14 := time.Now()

	// Apply the transaction to the current state (included in the env)
	// Double clean the trace to prevent duplications
	mongo.TraceGlobal.Reset()
	_, gas, failed, err := ApplyMessage(vmenv, msg, gp)

	// print("applytransaction stage1.5 time is ", fmt.Sprintf("%s", time.Since(start_tempt14)) , "\n")
	// start_tempt15 := time.Now()

	// write trace to the array
	mongo.BashTrs[mongo.CurrentNum] = mongo.Trace{tx.Hash().Hex(), mongo.TraceGlobal.String()}

	if err != nil {
		return nil, 0, err
	}

	// Update the state with pending changes
	var root []byte
	if config.IsByzantium(header.Number) {
		statedb.Finalise(true)
	} else {
		root = statedb.IntermediateRoot(config.IsEIP158(header.Number)).Bytes()
	}
	*usedGas += gas

	// print("applytransaction stage1.6 time is ", fmt.Sprintf("%s", time.Since(start_tempt15)) , "\n")
	// start_tempt16 := time.Now()

	// Create a new receipt for the transaction, storing the intermediate root and gas used by the tx
	// based on the eip phase, we're passing whether the root touch-delete accounts.
	receipt := types.NewReceipt(root, failed, *usedGas)
	receipt.TxHash = tx.Hash()
	receipt.GasUsed = gas
	// if the transaction created a contract, store the creation address in the receipt.
	if msg.To() == nil {
		receipt.ContractAddress = crypto.CreateAddress(vmenv.Context.Origin, tx.Nonce())
	}
	// Set the receipt logs and create a bloom for filtering
	receipt.Logs = statedb.GetLogs(tx.Hash())
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})
	receipt.BlockHash = statedb.BlockHash()
	receipt.BlockNumber = header.Number
	receipt.TransactionIndex = uint(statedb.TxIndex())

	// write receipt to the array
	mongo.BashRes[mongo.CurrentNum] = mongo.Rece{receipt.ContractAddress.String(), fmt.Sprintf("%d", receipt.CumulativeGasUsed),
			fmt.Sprintf("%d", receipt.GasUsed), fmt.Sprintf("0x%d", receipt.Status), receipt.TxHash.Hex(), mongo.TxVMErr}

	// print("apply the transaction, before mongodb ", fmt.Sprintf("%s", time.Since(start_tempt16)) , "\n")
	// start_tempt2 := time.Now()

	// bash write bash number of transactions, receipts and traces into the db
	if mongo.CurrentNum != mongo.BashNum - 1 {
		mongo.CurrentNum = mongo.CurrentNum + 1
	} else {
		// start := time.Now()
		db_tx := mongo.SessionGlobal.DB("geth").C("transaction")
		if db_tx == nil {
			var recon_err error
			mongo.SessionGlobal, recon_err = mgo.Dial("")
			if recon_err != nil {
				print("Error in tx")
				panic(recon_err)
			}
			db_tx = mongo.SessionGlobal.DB("geth").C("transaction")
		}
		
		session_err := db_tx.Insert(mongo.BashTxs...)
		if session_err != nil {
			mongo.SessionGlobal.Refresh()
			for i := 0; i < mongo.BashNum; i++ {
				 session_err = db_tx.Insert(&mongo.BashTxs[i]) 
				 if session_err != nil {
					json_tx, json_err := json.Marshal(&mongo.BashTxs[i])
					if json_err != nil {
						mongo.ErrorFile.WriteString(fmt.Sprintf("Transaction;%s;%s\n", mongo.BashTxs[i].(mongo.Transac).Tx_Hash, json_err))
					}
					mongo.ErrorFile.WriteString(fmt.Sprintf("Transaction|%s|%s\n", json_tx, session_err))
			      }
			 }
		}

		db_tr := mongo.SessionGlobal.DB("geth").C("trace")
		if db_tr == nil {
			var recon_err error
                        mongo.SessionGlobal, recon_err = mgo.Dial("")
                        if recon_err != nil {
                                print("Error in tr")
				panic(recon_err)
                        }
			db_tr = mongo.SessionGlobal.DB("geth").C("trace")
		}
		
		session_err = db_tr.Insert(mongo.BashTrs...)
		if session_err != nil {
			mongo.SessionGlobal.Refresh()
			for i := 0; i < mongo.BashNum; i++ {
				session_err = db_tr.Insert(&mongo.BashTrs[i])
				if session_err != nil {
					json_tr, json_err := json.Marshal(&mongo.BashTrs[i]) 
					if json_err != nil {
						mongo.ErrorFile.WriteString(fmt.Sprintf("Trace;%s;%s\n", mongo.BashTrs[i].(mongo.Trace).Tx_Hash, json_err))
					}
					mongo.ErrorFile.WriteString(fmt.Sprintf("Trace|%s|%s \n", json_tr, session_err))
				 }
			}	
		}

		db_re := mongo.SessionGlobal.DB("geth").C("receipt")
		if db_re == nil {
			var recon_err error
                        mongo.SessionGlobal, recon_err = mgo.Dial("")
                        if recon_err != nil {
				print("Error in re")
                                panic(recon_err)
                        }
			db_re = mongo.SessionGlobal.DB("geth").C("receipt")
		}

		session_err = db_re.Insert(mongo.BashRes...)
		if session_err != nil {
			mongo.SessionGlobal.Refresh()
			for i := 0; i < mongo.BashNum; i++ {
				session_err = db_re.Insert(&mongo.BashRes[i])
				if session_err != nil {
					json_re, json_err := json.Marshal(&mongo.BashRes[i])
					if json_err != nil {
						 mongo.ErrorFile.WriteString(fmt.Sprintf("Receipt;%s;%s\n", mongo.BashRes[i].(mongo.Rece).Re_TxHash, json_err))
					}
					mongo.ErrorFile.WriteString(fmt.Sprintf("Receipt|%s|%s\n", json_re, session_err))
				}
			}
		}

		mongo.CurrentNum = 0
	}

	// print("apply the transaction after mongodb time is ", fmt.Sprintf("%s", time.Since(start_tempt2)) , "\n")

	return receipt, gas, err
}
