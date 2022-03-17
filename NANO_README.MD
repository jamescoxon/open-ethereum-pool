### Setup Server
Need at least 300GB of SSD, advised current setup: SSD 500Gb + HDD 1Tb

### Install dependencies:
	sudo apt install build-essential python3-dev python3-pip redis git
  
### Install Go https://go.dev/doc/install
    export PATH=$PATH:/usr/local/go/bin

### Install Geth (https://geth.ethereum.org/docs/install-and-build/installing-geth)
	go get -d github.com/ethereum/go-ethereum
	go install github.com/ethereum/go-ethereum/cmd/geth


### Sync Geth 
(https://blog.ethereum.org/2019/07/10/geth-v1-9-0/ - this is how to use the 'freezer' option to use the HDD for old data, note that sync can take 48hrs)

  `geth --datadir.ancient /mnt/hdd/ console`

### Setup Eth account:
	geth account new

### Install Pippin (Nano Wallet)
	pip3 install --user pippin-wallet
	cd .local/bin/
	./pippin-server --generate-config
	cd
	cd PippinData
	cp sample.config.yaml config.yaml
	nano config.yml

### Edit config.yaml:
	Uncomment and update node_rpc_url: https://rainstorm.city/api
	Uncomment and update node_ws_url: wss://rainstorm.city/websocket
	Uncomment and update work_peers

  `curl -g -d '{"action":"wallet_create"}' localhost:11338`
  
  `curl -g -d '{"action":"account_create", "wallet" : "7e72e2f5-44b2-46f5-ad8f-9d07266aa33b"}' localhost:11338`

### Install open-ethereum-pool:
	git config --global http.https://gopkg.in.followRedirects true
	git clone https://github.com/jamescoxon/open-ethereum-pool.git
	cd open-ethereum-pool
	make
    cp config.example.json config.json
    change redis database to 1
    poolfee 0
    donate false
    payouts
    	enabled true
    	address <public address>

### Setup firewall
	apt install ufw
	ufw allow ssh
	ufw allow 8008
	ufw allow 8080
	ufw enable

### Sign up to Kraken
	Obtain API key
	Verify for higher payouts (otherwise when you mine a block it won't allow you to fully withdraw your Nano and crash the script!)



### Example nano configuration
```
        "payouts": {
                "enabled": true,
                "requirePeers": 25,
                "interval": "120m",
                "daemon": "http://127.0.0.1:8545", //This is your geth node
                "timeout": "10s",
                "address": "0x445Da00c8B326d54685F1c31E7ea54d78B6a665a",
                "gas": "21000",
                "gasPrice": "50000000000",
                "autoGas": true,
                "threshold": 50,
                "nanothreshold": 500, //This is your theshold for miners using Nano
                "bgsave": false,
                "pippinwallet" : "http://localhost:11338", //Pippin wallet local address
                "hotnanowallet" : "nano_3x1mke9dbs8t88i5bpbn5wabh9qq3hpr361r6fgtqyr5hz9dusacxuijq6nk", //Your Nano hot wallet (where the exchange withdraws to)
                "nanowallet" : "68296766-a016-4db6-8f21-e83571716704", //This is your Pippin Wallet identifier
                "exchangeethdeposit" : "", //Leave this blank and the pool will detect the eth deposit address for the exchange
                "exchangekey" : "", //Your kraken exchange key
                "exchangesecret" : "", //Your kraken exchange secret
                "nanopayout" : true,
                "dustaccount" : "nano_1gxx3dbrprrh9ycf1p5wo9qgmftppg6z7688njum14aybjkaiweqmwpuu9py", //Any spare Nano gets donated
                "nanoenabled" : true,
                "nanoexchange" : "kraken",
                "nanofaucet" : false, //You can set the pool up to regularly send out Nano as a small faucet reward
                "faucetnanowallet" : "nano_1nsncy9z3rj6ug35ns9amy5kjou6ycyjk3qkwkfq1zthdi8qyys5dmk31kg1"

        },
```
