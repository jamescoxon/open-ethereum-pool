package nano_payouts

import (
	"fmt"
	"log"
	"math/big"
	"time"
	"strings"
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"errors"

	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/sammy007/open-ethereum-pool/rpc"
	"github.com/sammy007/open-ethereum-pool/storage"
	"github.com/sammy007/open-ethereum-pool/util"

	krakenapi "github.com/jamescoxon/kraken-go-api-client"
)

type PayoutsConfig struct {
	Enabled      bool   `json:"enabled"`
	RequirePeers int64  `json:"requirePeers"`
	Interval     string `json:"interval"`
	Daemon       string `json:"daemon"`
	Timeout      string `json:"timeout"`
	Address      string `json:"address"`
	Gas          string `json:"gas"`
	GasPrice     string `json:"gasPrice"`
	AutoGas      bool   `json:"autoGas"`
	// In Shannon
	Threshold       int64  `json:"threshold"`
	BgSave          bool   `json:"bgsave"`
	// Nano Sections
	NanoThreshold       int64  `json:"nanothreshold"`
	HotNanoWallet   string `json:"hotnanowallet"`
	NanoExchange   string `json:"nanoexchange"`
	ExchangeDeposit string `json:"exchangeethdeposit"`
	ExchangeKey     string `json:"exchangekey"`
	ExchangeSecret  string `json:"exchangesecret"`
	NanoPayout      bool   `json:"nanopayout"`
	NanoWallet      string   `json:"nanowallet"`
	DustAccount      string   `json:"dustaccount"`
	NanoEnabled      bool   `json:"nanoenabled"`
	NanoFaucet      bool   `json:"nanofaucet"`
	PippinWallet      string   `json:"pippinwallet"`
	FaucetNanoWallet   string `json:"faucetnanowallet"`

}

func (c PayoutsConfig) GasHex() string {
	x := util.String2Big(c.Gas)
	return hexutil.EncodeBig(x)
}

func (c PayoutsConfig) GasPriceHex() string {
	x := util.String2Big(c.GasPrice)
	return hexutil.EncodeBig(x)
}

type PayoutsProcessor struct {
	config   *PayoutsConfig
	backend  *storage.RedisClient
	rpc      *rpc.RPCClient
	halt     bool
	lastFail error
}

func NewPayoutsProcessor(cfg *PayoutsConfig, backend *storage.RedisClient) *PayoutsProcessor {
	u := &PayoutsProcessor{config: cfg, backend: backend}
	u.rpc = rpc.NewRPCClient("PayoutsProcessor", cfg.Daemon, cfg.Timeout)
	return u
}

func (u *PayoutsProcessor) getDepositAddress() (string, error) {

	api := krakenapi.New(u.config.ExchangeKey, u.config.ExchangeSecret)

	// Identify the deposit address (if pre-configured in json use that otherwise poll api)
	exchangeDepositAddresses := ""
	if u.config.ExchangeDeposit == "" {
		depositAddresses, err := api.DepositAddresses("XETH", "Ethereum (ERC20)")
		if err == nil {
			log.Println("Nano - Deposit Address:", (*depositAddresses)[0].Address)
			exchangeDepositAddresses = (*depositAddresses)[0].Address
		} else {
			return "", errors.New("Failed to Get Deposit Address")
		}

	} else {
		exchangeDepositAddresses = u.config.ExchangeDeposit
	}

	return exchangeDepositAddresses, nil
}

func (u *PayoutsProcessor) getExchangeBalance(currency string) (float64, error) {

	if u.config.NanoExchange == "kraken" {
		api := krakenapi.New(u.config.ExchangeKey, u.config.ExchangeSecret)

		balance, err := api.Balance()
		if err != nil {
			return 0.0, nil
		}

		if currency == "XETH" {
			return (*balance).XETH, nil
		}

		if currency == "NANO" {
			return (*balance).NANO, nil
		}
		return 0.0, nil
	} else {
		log.Println("Nano - No Exchange Selected")
		return 0.0, errors.New("No Exchange Selected")
	}


}

func (u *PayoutsProcessor) getExchangeTicker(ticker string) (string, error) {

	api := krakenapi.New(u.config.ExchangeKey, u.config.ExchangeSecret)

	result, err := api.Query("Ticker", map[string]string{
		"pair": ticker,
	})
	if err != nil {
		log.Println("Nano - ", err)
		return "", err
	}

	nanoeth := result.(map[string]interface{})["NANOETH"]
	nanoethA := nanoeth.(map[string]interface{})["a"]
	nanoEthEx := nanoethA.([]interface{})[0]
	return nanoEthEx.(string), nil
}

func (u *PayoutsProcessor) nanoReceiveAll(wallet string) (string, error) {
	values := map[string]string{"action": "receive_all", "wallet": wallet}
	json_data, err := json.Marshal(values)

    	if err != nil {
        	log.Println("Nano - ", err)
    	}
    	resp, err := http.Post(u.config.PippinWallet, "application/json",
        	bytes.NewBuffer(json_data))

	if err != nil {
		log.Println("Nano - ", err)
    	}

    	var res map[string]interface{}

    	json.NewDecoder(resp.Body).Decode(&res)

	return wallet, nil
}

func (u *PayoutsProcessor) nanoAccountBalance(account string) (string, error) {
	values := map[string]string{"action": "account_balance", "account": account, "include_only_confirmed" : "true"}
	json_data, err := json.Marshal(values)

    	if err != nil {
        	log.Println("Nano - ", err)
    	}

    	resp, err := http.Post(u.config.PippinWallet, "application/json",
        	bytes.NewBuffer(json_data))

	if err != nil {
		log.Println("Nano - ", err)
    	}

    	var res map[string]interface{}

    	json.NewDecoder(resp.Body).Decode(&res)

	return res["balance"].(string), nil

}

func (u *PayoutsProcessor) nanoSend(destination string, amount string, wallet string, source string) (string, error) {
	timestamp := strconv.FormatInt(time.Now().UnixNano(), 10)
	values := map[string]string{"action": "send", "wallet": wallet, "source" : source, "destination" : destination, "amount" : amount, "id" : timestamp}
	json_data, err := json.Marshal(values)

    	if err != nil {
        	log.Println("Nano - ", err)
    	}

    	resp, err := http.Post(u.config.PippinWallet, "application/json",
        	bytes.NewBuffer(json_data))

	if err != nil {
		log.Println("Nano -", err)
    	}

    	var res map[string]interface{}

    	json.NewDecoder(resp.Body).Decode(&res)

	if val, ok := res["block"]; ok {
		return val.(string), nil
	} else {
		log.Println("Nano - Error", res)
		return "error", err
	}

}

func (u *PayoutsProcessor) Start() {
	log.Println("Nano - Starting Nano payouts")

	intv := util.MustParseDuration(u.config.Interval)
	timer := time.NewTimer(intv)
	log.Printf("Nano - Set payouts interval to %v", intv)

	locked, err := u.backend.IsPayoutsLocked()
	if err != nil {
		log.Println("Nano - Unable to start payouts:", err)
		return
	}
	if locked {
		log.Println("Nano - Unable to start payouts because they are locked")
		return
	}

	// Immediately process payouts after start
	if u.config.NanoFaucet {
		u.faucet()
	}
	u.process()
	timer.Reset(intv)

	go func() {
		for {
			select {
			case <-timer.C:
				if u.config.NanoFaucet {
					u.faucet()
				}
				u.process()
				timer.Reset(intv)
			}
		}
	}()
}

func (u *PayoutsProcessor) faucet() {

	if u.config.NanoEnabled == false {
		return
	}
	result, err := u.nanoReceiveAll(u.config.NanoWallet)
	if err != nil {
		log.Println("Nano Faucet - ", result, err)
		u.lastFail = err
		return
	}
	payees, err := u.backend.GetPayees()
	if err != nil {
		log.Println("Error while retrieving payees from backend:", err)
		return
	}

	for _, login := range payees {

		// ignore any eth strings as we process these seperately
		if !strings.HasPrefix(login, "nano_") {
			log.Println("Ignore Eth address:", login)
			continue
		}

		// Send amount
		nanoAmountToSend := "1000000000000000000000000000"
		txHash, err := u.nanoSend(login, nanoAmountToSend, u.config.NanoWallet, u.config.FaucetNanoWallet)
		if err != nil {
			log.Println("Nano Faucet - ", result, err)
			u.lastFail = err
			return
		}
		log.Println("Nano Faucet - ", txHash, err)
}
}

func (u *PayoutsProcessor) calculateNano(amountInShannon *big.Int, totalShannon *big.Int, nanoDistributionBalanceF *big.Float) (*big.Int) {

                // Calculate % of total
                shareShannon := new(big.Float)
                onehundred := big.NewFloat(100.0)

		amountInShannonF := new(big.Float).SetInt(amountInShannon)
		totalShannonF := new(big.Float).SetInt(totalShannon)

                // 1) Divide shannon / totalShannon
                shareShannon.Quo(amountInShannonF, totalShannonF)
                shareShannon.Mul(shareShannon, onehundred)
                log.Println("Nano - Split:", amountInShannon, totalShannon, shareShannon)

                // Calculate amount of Nano (% of withdrawn amount)
                nanoAmountToSendF := new(big.Float)
                // 2) (nano_total / 100) * share
                nanoDistributionBalanceF.Quo(nanoDistributionBalanceF, onehundred)
                nanoAmountToSendF.Mul(nanoDistributionBalanceF, shareShannon)

		nanoAmountToSend := new(big.Int)
		nanoAmountToSendF.Int(nanoAmountToSend)

                log.Println("Nano - calculate:", nanoAmountToSend, shareShannon, nanoDistributionBalanceF)

		return nanoAmountToSend
}

func (u *PayoutsProcessor) pollExchangeForDeposit(amountInWeiFloat *big.Float) (*big.Float, error) {
        // Poll the XETH exchange balance every 10 seconds until the XETH balance is >= to the amount sent.

	ethWeiConvert := big.NewFloat(1000000000000000000.0)
        exchangeBalanceWei := big.NewFloat(0)
        loopcount := 0
        for {
                balanceETH := 0.0
                balanceETH, err := u.getExchangeBalance("XETH")
                if err != nil {
                        u.halt = true
                        u.lastFail = err
                        return exchangeBalanceWei, err
                }
		currentEthBalanceF := new(big.Float)
                currentEthBalanceF.SetFloat64(balanceETH)

                log.Println("Nano - ", balanceETH)

                exchangeBalanceWei.Mul(currentEthBalanceF, ethWeiConvert)

                log.Println("Nano - Exchange Balance:", currentEthBalanceF, exchangeBalanceWei, amountInWeiFloat)
                // TODO this is the switch for when we go live on a pool, currently it doesn't compare properly.
                //if exchangeBalanceWei.Cmp(amountInWeiFloat) > -1 {
                //      break
                //}

                if exchangeBalanceWei.Cmp(big.NewFloat(100000000000000.0)) > 0 {
                        break
                }
                if loopcount > 180 { //10seconds * 6 * 30
                        log.Println("Nano - Exchange Balance Error: To slow")
                        u.halt = true
                        u.lastFail = errors.New("Exchange Deposit To Slow > 30 mins")
                        return exchangeBalanceWei, err
                }

                loopcount++
                time.Sleep(10 * time.Second)
        }
	return exchangeBalanceWei, nil
}

func (u *PayoutsProcessor) process() {
	if u.halt {
		log.Println("Nano - Nano Payments suspended due to last critical error:", u.lastFail)
		return
	}

	if u.config.NanoEnabled == false {
		return
	}

	mustPay := 0
	minersPaid := 0

	totalShannon := big.NewInt(0)
	numberOfAccounts := 0

	totalAmount := big.NewInt(0)
	payees, err := u.backend.GetPayees()
	if err != nil {
		log.Println("Error while retrieving payees from backend:", err)
		return
	}

	// Scan all the payees, ignore everything that isn't a nano address, if it has reached the payout threshold added it to the pool
	for _, login := range payees {
		amount, _ := u.backend.GetBalance(login)
		amountInShannon := big.NewInt(amount)

		// ignore any eth strings as we process these seperately
		if !strings.HasPrefix(login, "nano_") {
			log.Println("Ignore Eth address:", login)
			continue
		}
		// TODO - unsure whether this is sensible, if there is some sort of failure is it better to rollback the pending balance 
		// back into balance and then start again?

                pending, _ := u.backend.GetPending(login)
                u.backend.RollbackBalance(login, pending)

		if !u.reachedNanoThreshold(amountInShannon) {
			continue
		}

		// Debit miner's balance and update stats - this moves their balance into pending
		err = u.backend.UpdateBalance(login, amount)
		if err != nil {
			log.Printf("Failed to update balance for %s, %v Shannon: %v", login, amount, err)
			u.halt = true
			u.lastFail = err
			break
		}

		totalShannon.Add(totalShannon, amountInShannon)
		numberOfAccounts++
	}

	log.Println("Nano - TotalShannon: ", totalShannon)

	// Shannon^2 = Wei
	// Calculate the 'pooled' eth in Wei, if this is a Nano payout only pool this should be all the funds
	amountInWei := new(big.Int).Mul(totalShannon, util.Shannon)

	log.Println("Nano - NumberOfAccounts: ", numberOfAccounts)

	if numberOfAccounts == 0 {
		log.Println("No accounts to credit")
		return
	}

	// Identify the deposit address (if pre-configured in json use that otherwise poll api)
	exchangeDepositAddresses, err := u.getDepositAddress()
	if err != nil {
		log.Println("Nano - Failed to identify deposit address")
		u.halt = true
		u.lastFail = err
		return
	}

	// Require active peers before processing
	if !u.checkPeers() {
		log.Println("Nano - No Peers, failing")
		return
	}

	// Require unlocked account
	if !u.isUnlockedAccount() {
		log.Println("Nano - Account not unlocked, failing")
		return
	}

	// Check if we have enough funds
	poolBalance, err := u.rpc.GetBalance(u.config.Address)
	if err != nil {
		log.Println("Nano - Insufficient Balance")
		u.halt = true
		u.lastFail = err
		return
	}

	if poolBalance.Cmp(amountInWei) < 0 {
		log.Println("Nano - not enough balance for payment")
		err := fmt.Errorf("not enough balance for payment, need %s Wei, pool has %s Wei",
			amountInWei.String(), poolBalance.String())
		u.halt = true
		u.lastFail = err

		return
	}

	value := hexutil.EncodeBig(amountInWei)
	log.Println("Nano - Starting Main Payout")

	// If Nano Payout enabled send the 'pooled' eth to the exchange
	if u.config.NanoPayout {
		txHash, err := u.rpc.SendTransaction(u.config.Address, exchangeDepositAddresses, u.config.GasHex(), u.config.GasPriceHex(), value, u.config.AutoGas)
		if err != nil {
			log.Printf("Failed to send payment to %s, %v Shannon: %v. Check outgoing tx for %s in block explorer and docs/PAYOUTS.md",
				exchangeDepositAddresses, totalShannon, err, exchangeDepositAddresses)
			u.halt = true
			u.lastFail = err
			return
		}

		log.Println("Nano - Sent:", txHash)
	} else {
		log.Println("Nano - Send to Exchange not authorised, please review config.json")
		u.halt = true
		u.lastFail = err
		return
	}

	log.Println("Nano - Waiting for Deposit on Exchange")

	currentEthBalanceF := new(big.Float)
	amountInWeiFloat := new(big.Float).SetInt(amountInWei)

	//Poll the exchange while waiting for the deposit
	currentEthBalanceF, err = u.pollExchangeForDeposit(amountInWeiFloat)
	if err != nil {
		log.Println("Nano - Error polling exchange balance")
		u.halt = true
		u.lastFail = err
		return
	}

	api := krakenapi.New(u.config.ExchangeKey, u.config.ExchangeSecret)

	// Get Ticker
	nanoEthEx, err := u.getExchangeTicker("NANOETH")
	nanoEthExF := new(big.Float)
	nanoEthExF.SetString(nanoEthEx)

	if err != nil {
		log.Println("Nano - NANOETH: Error")
		u.halt = true
		u.lastFail = err
		return
	}

	// Calculate the amount of NANO to buy (current XETH balance / rate)
	balanceETH, err := u.getExchangeBalance("XETH")
	if err != nil {
        	u.halt = true
                u.lastFail = err
		return
	}
	currentEthBalanceF.SetFloat64(balanceETH)

	amountNanoToBuyF := new(big.Float)
	amountNanoToBuyF.Quo( currentEthBalanceF, nanoEthExF)
	log.Println("Nano - Buy:", amountNanoToBuyF)

	// Trade XETH to NANO
	exchangeTrade, err := api.AddOrder("NANOETH", "buy", "market", amountNanoToBuyF.String(), map[string]string{"validate": "false"} )

	if err != nil {
		log.Println("Nano - Trade Error: ", err)
		u.halt = true
		u.lastFail = err
		return
	}
	log.Println("Nano - Trade:", exchangeTrade)

	// Wait 5 seconds just to make sure order has started to process
	time.Sleep(5 * time.Second)

	currentNANOBalanceF := new(big.Float)

	loopcount := 0
	for {
		balanceNANO, err := u.getExchangeBalance("NANO")
		if err != nil {
			u.halt = true
			u.lastFail = err
			return
		}

		currentNANOBalanceF.SetFloat64(balanceNANO)
		emptyBalance := big.NewFloat(0.0)
		if currentNANOBalanceF.Cmp(emptyBalance) == 1 {
			break
		}
		log.Println("Nano - current balance:", currentNANOBalanceF)

		if loopcount > 180 { //10seconds * 6 * 30
			log.Println("Nano - Exchange Balance Error: To slow")
			u.halt = true
			u.lastFail = errors.New("Exchange Deposit To Slow > 30 mins")
			return
		}
		loopcount++
		time.Sleep(10 * time.Second)

	}
	log.Println("Nano - Withdraw Nano to Hot Wallet", u.config.HotNanoWallet, currentNANOBalanceF)

	// Withdraw Nano to predefined Nano Address (this is 'key' and is set directly on Kraken Website)
        withdrawResult, err := api.Query("Withdraw", map[string]string{
                "asset": "NANO",
		"key" : "hotwallet",
		"amount" : currentNANOBalanceF.String(),
       })

	log.Println("Nano - Withdraw:", withdrawResult, err)

	nanoDistributionBalanceI := new(big.Int)
	nanoDistributionBalanceF := new(big.Float)

	loopcount = 0
	for {
	        result, err := u.nanoReceiveAll(u.config.NanoWallet)
		if err != nil {
			log.Println("Nano - ", result, err)
			u.halt = true
			u.lastFail = err
			return
		}

		balance, err := u.nanoAccountBalance(u.config.HotNanoWallet)
		if err != nil {
			log.Println("Nano - ", balance, err)
			u.halt = true
			u.lastFail = err
			return
		}

		nanoDistributionBalanceI.SetString(balance, 10)
		nanoDistributionBalanceF.SetInt(nanoDistributionBalanceI)

		if nanoDistributionBalanceF.Cmp(currentNANOBalanceF) >= 0 {
			break
		}

		if loopcount > 180 { //10seconds * 6 * 30
			log.Println("Nano - Exchange Balance Error: To slow")
			u.halt = true
			u.lastFail = errors.New("Exchange Deposit To Slow > 30 mins")
			return
		}
		loopcount++

		time.Sleep(10 * time.Second)
	}

	log.Println("Nano - Distribute to Users")

        // Scan all the payees, ignore everything that isn't a nano address
        for _, login := range payees {
                pending, _ := u.backend.GetPending(login)
                amountInShannon := big.NewInt(pending)

                // ignore any eth strings as we process these seperately
                if !strings.HasPrefix(login, "nano_") {
                        log.Println("Ignore Eth address:", login)
                        continue
                }

		// Check if above threshold
		if !u.reachedNanoThreshold(amountInShannon) {
			continue
		}

		mustPay++

		nanoAmountToSend := new(big.Int)
		nanoAmountToSend = u.calculateNano(amountInShannon, totalShannon, nanoDistributionBalanceF)

                accountBalance, err := u.nanoAccountBalance(u.config.HotNanoWallet)
                if err != nil {
                        log.Println("Nano - ", accountBalance, err)
                        u.halt = true
                        u.lastFail = err
                        return
                }
		nanoDistributionBalanceI.SetString(accountBalance, 10)

		// Check if we have sufficient balance
		if nanoDistributionBalanceI.Cmp(nanoAmountToSend) == -1 {
			log.Println("Nano - insufficient balance, use balance as last payout, may need to be checked")
			nanoAmountToSend.Set(nanoDistributionBalanceI)
		}

		log.Println("Nano - Sending:", nanoAmountToSend, login)

		// Send amount
		txHash, err := u.nanoSend(login, nanoAmountToSend.String(), u.config.NanoWallet, u.config.HotNanoWallet)

		if err != nil {
			return
		}
		// Update storge (WritePayment)
		// Log transaction hash
		err = u.backend.WritePayment(login, txHash, pending)
		if err != nil {
			log.Printf("Failed to log payment data for %s, %v Shannon, tx: %s: %v", login, pending, txHash, err)
			u.halt = true
			u.lastFail = err
			break
		}

	}

	time.Sleep(10 * time.Second)

	// Don't want to leave dust in the account, as its Nano we can move it!
	dustbalance, err := u.nanoAccountBalance(u.config.HotNanoWallet)
	log.Println("Nano - Dust Balance:", dustbalance, err)
	if err != nil {
		log.Println("Nano - ", dustbalance, err)
		return
	}
	if dustbalance != "0" {
		log.Println("Nano - Sending dust")
		u.nanoSend(u.config.DustAccount, dustbalance, u.config.NanoWallet, u.config.HotNanoWallet)
	}

	if mustPay > 0 {
		log.Printf("Nano - Paid total %v Shannon to %v of %v payees", totalAmount, minersPaid, mustPay)
	} else {
		log.Println("Nano - No payees that have reached payout threshold")
	}

	// Save redis state to disk
	if minersPaid > 0 && u.config.BgSave {
		u.bgSave()
	}
}

func (p PayoutsProcessor) isUnlockedAccount() bool {
	_, err := p.rpc.Sign(p.config.Address, "0x0")
	if err != nil {
		log.Println("Unable to process payouts:", err)
		return false
	}
	return true
}

func (p PayoutsProcessor) checkPeers() bool {
	n, err := p.rpc.GetPeerCount()
	if err != nil {
		log.Println("Unable to start payouts, failed to retrieve number of peers from node:", err)
		return false
	}
	if n < p.config.RequirePeers {
		log.Println("Unable to start payouts, number of peers on a node is less than required", p.config.RequirePeers)
		return false
	}
	return true
}

func (p PayoutsProcessor) reachedNanoThreshold(amount *big.Int) bool {
	return big.NewInt(p.config.NanoThreshold).Cmp(amount) < 0
}

func formatPendingPayments(list []*storage.PendingPayment) string {
	var s string
	for _, v := range list {
		s += fmt.Sprintf("\tAddress: %s, Amount: %v Shannon, %v\n", v.Address, v.Amount, time.Unix(v.Timestamp, 0))
	}
	return s
}

func (p PayoutsProcessor) bgSave() {
	result, err := p.backend.BgSave()
	if err != nil {
		log.Println("Failed to perform BGSAVE on backend:", err)
		return
	}
	log.Println("Saving backend state to disk:", result)
}
