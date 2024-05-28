package comms

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestSetupLightingComms(t *testing.T) {
	// setup

	ctx := context.Background()

	net, err := network.New(ctx,
		network.WithCheckDuplicate(),
		network.WithAttachable(),
		// Makes the network internal only, meaning the host machine cannot access it.
		// Remove or use `network.WithDriver("bridge")` to change the network's mode.
		// network.WithInternal(),
		network.WithDriver("bridge"),
		// network.WithLabels(map[string]string{"this-is-a-test": "value"}),
	)

	if err != nil {
		log.Fatalln("Error: ", err)
		return
	}

	// Create bitcoind regtest node
	reqbtcd := testcontainers.ContainerRequest{
		Image:        "polarlightning/bitcoind:26.0",
		Name:         "bitcoindbackend",
		WaitingFor:   wait.ForLog("Initialized HTTP server"),
		ExposedPorts: []string{"18443/tcp", "18444/tcp", "28334/tcp", "28335/tcp", "28336/tcp"},
		Networks:     []string{net.Name},

		Cmd: []string{"bitcoind", "-server=1", "-regtest=1", "-rpcuser=rpcuser", "-rpcpassword=rpcpassword", "-debug=1", "-zmqpubrawblock=tcp://0.0.0.0:28334", "-zmqpubrawtx=tcp://0.0.0.0:28335", "-zmqpubhashblock=tcp://0.0.0.0:28336", "-txindex=1", "-dnsseed=0", "-upnp=0", "-rpcbind=0.0.0.0", "-rpcallowip=0.0.0.0/0", "-rpcport=18443", "-rest", "-listen=1", "-listenonion=0", "-fallbackfee=0.0002", "-blockfilterindex=1", "-peerblockfilters=1"},
	}

	btcdC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: reqbtcd,
		Started:          true,
	})

	if err != nil {
		t.Fatalf("could not setup bitcoind %+v", err)
	}

	btcdIP, err := btcdC.ContainerIP(ctx)

	if err != nil {
		t.Fatalf("could not get ContainerIP %+v", err)
	}


    log.Println("btcdIP: ", btcdIP)

	_, _, err = btcdC.Exec(ctx, []string{"bitcoin-cli", "-regtest", "-rpcuser=rpcuser", "-rpcpassword=rpcpassword", "createwallet", "wallet"})
	if err != nil {
		t.Errorf("could not create wallet  %+v", err)
	}

	_, _, err = btcdC.Exec(ctx, []string{"bitcoin-cli", "-regtest", "-rpcuser=rpcuser", "-rpcpassword=rpcpassword", "-generate", "101"})

	if err != nil {
		t.Fatalf("could not create blocks  %+v", err)
	}

	// create Alice node LND
	reqlndAlice := testcontainers.ContainerRequest{
		Image:        "polarlightning/lnd:0.17.5-beta",
		WaitingFor:   wait.ForLog("Server listening on"),
		ExposedPorts: []string{"18445/tcp", "10009/tcp"},

		Networks: []string{net.Name},
		Cmd:      []string{"lnd", "--noseedbackup", "--trickledelay=5000", "--alias=alice" /* "--externalip=alice", */, "--tlsextradomain=alice", "--tlsextradomain=host.docker.bridge", "--tlsextradomain=host.docker.internal", "--listen=0.0.0.0:9735", "--rpclisten=0.0.0.0:10009", "--restlisten=0.0.0.0:8080", "--bitcoin.active", "--bitcoin.regtest", "--bitcoin.node=bitcoind", "--bitcoind.rpchost=" + btcdIP, "--bitcoind.rpcuser=rpcuser", "--bitcoind.rpcpass=rpcpassword", "--bitcoind.zmqpubrawblock=tcp://" + btcdIP + ":28334", "--bitcoind.zmqpubrawtx=tcp://" + btcdIP + ":28335"},
	}

	lndAliceC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: reqlndAlice,
		Started:          true,
	})



	if err != nil {
		t.Fatalf("could not create Alice lnd container  %+v", err)
	}

	_, addressReader, err := lndAliceC.Exec(ctx, []string{"lncli", "--tlscertpath", "/home/lnd/.lnd/tls.cert", "--macaroonpath", "home/lnd/.lnd/data/chain/bitcoin/regtest/admin.macaroon", "newaddress", "p2tr"})

	reader := io.Reader(addressReader)
	buf := make([]byte, 1024)

	type LndAddress struct {
		Address string
	}

	var address LndAddress
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			index := strings.Index(string(buf[:n]), "{")
			err := json.Unmarshal(buf[index:n], &address)
			if err != nil {
				log.Fatalln("json.Unmarshal: ", err)
			}

			fmt.Printf("adress: %+v ", address)
		}
		if err != nil {
			break
		}
	}
    

	// fund Alice node
	_, _, err = btcdC.Exec(ctx, []string{"bitcoin-cli", "-regtest", "-rpcuser=rpcuser", "-rpcpassword=rpcpassword", "sendtoaddress", address.Address, "10"})

	if err != nil {
		t.Errorf("could not fund Alice's wallet  %+v", err)
	}


	_, _, err = btcdC.Exec(ctx, []string{"bitcoin-cli", "-regtest", "-rpcuser=rpcuser", "-rpcpassword=rpcpassword", "-generate", "5"})

	if err != nil {
		t.Fatalf("could not create blocks  %+v", err)
	}

	_, _, err = lndAliceC.Exec(ctx, []string{"lncli", "--tlscertpath", "/home/lnd/.lnd/tls.cert", "--macaroonpath", "home/lnd/.lnd/data/chain/bitcoin/regtest/admin.macaroon", "listunspent"})

	if err != nil {
		t.Errorf("could not check balance  %+v ", err)
	}


	// create bob node LND

	reqLndBob := testcontainers.ContainerRequest{
		Image:        "polarlightning/lnd:0.17.5-beta",
		WaitingFor:   wait.ForLog("Server listening on"),
		ExposedPorts: []string{"18446/tcp", "9736/tcp", "10009/tcp", "8080/tcp"},

		Networks: []string{net.Name},
		Cmd:      []string{"lnd", "--noseedbackup", "--trickledelay=5000", "--alias=bob" /* "--externalip=alice", */, "--tlsextradomain=bob", "--tlsextradomain=host.docker.bridge", "--tlsextradomain=host.docker.internal", "--listen=0.0.0.0:9736", "--rpclisten=0.0.0.0:10009", "--restlisten=0.0.0.0:8080", "--bitcoin.active", "--bitcoin.regtest", "--bitcoin.node=bitcoind", "--bitcoind.rpchost=" + btcdIP, "--bitcoind.rpcuser=rpcuser", "--bitcoind.rpcpass=rpcpassword", "--bitcoind.zmqpubrawblock=tcp://" + btcdIP + ":28334", "--bitcoind.zmqpubrawtx=tcp://" + btcdIP + ":28335"},
	}

	LndBobC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: reqLndBob,
		Started:          true,
	})

	lndBobIp, err := LndBobC.ContainerIP(ctx)

	if err != nil {
		t.Fatalf("could not get lndAliceC.ContainerIP %+v", err)
	}


    _, getInfoBobReader, err := LndBobC.Exec(ctx, []string{"lncli", "--tlscertpath", "/home/lnd/.lnd/tls.cert", "--macaroonpath", "home/lnd/.lnd/data/chain/bitcoin/regtest/admin.macaroon", "getinfo"})
	if err != nil {
		t.Errorf("could not get nodeInfo  %+v ", err)
	}


    reader = io.Reader(getInfoBobReader)
    buf = make([]byte, 3024)

    type NodeInfo struct {
        IdentityPubkey string `json:"identity_pubkey"`
        NumPeers int `json:"num_peers"`
        NumPendingChannels int `json:"num_pending_channels"`
        NumActiveChannels int `json:"num_active_channels"`
        NumInactiveChannels int `json:"num_inactive_channels"`
    }

	var bobInfo NodeInfo
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			index := strings.Index(string(buf[:n]), "{")
			err := json.Unmarshal(buf[index:n], &bobInfo)
			if err != nil {
				log.Fatalln("json.Unmarshal: ", err)
			}
		}
		if err != nil {
			break
		}
	}

	// open channel between Alice and Bob
    connectionStr := bobInfo.IdentityPubkey + "@" + lndBobIp + ":" + "9736"
    _, connectPeerReader, err := lndAliceC.Exec(ctx, []string{"lncli", "--tlscertpath", "/home/lnd/.lnd/tls.cert", "--macaroonpath", "home/lnd/.lnd/data/chain/bitcoin/regtest/admin.macaroon", "connect",connectionStr })

	if err != nil {
		t.Errorf("could not get nodeInfo  %+v ", err)
	}
    reader = io.Reader(connectPeerReader)
    buf = make([]byte, 1024)


    // connect Alice To Bob
	for {
		n, err := reader.Read(buf)
		if n > 0 {
            log.Println("connectPeerReader: ", string(buf[:n]))
			// index := strings.Index(string(buf[:n]), "{")
			// err := json.Unmarshal(buf[index:n], &info)
			// if err != nil {
			// 	log.Fatalln("json.Unmarshal: ", err)
			// }
		}
		if err != nil {
			break
		}
	}
	// open channel between Alice and Bob
    // connectionStr := bobInfo.IdentityPubkey + "@" + lndBobIp + ":" + "9736"
    _, openChannelReader, err := lndAliceC.Exec(ctx, []string{"lncli", "--tlscertpath", "/home/lnd/.lnd/tls.cert", "--macaroonpath", "home/lnd/.lnd/data/chain/bitcoin/regtest/admin.macaroon", "openchannel", "--node_key",  bobInfo.IdentityPubkey, "--fundmax" })

	if err != nil {
		t.Errorf("could not get nodeInfo  %+v ", err)
	}
    reader = io.Reader(openChannelReader)
    buf = make([]byte, 1024)


    // connect Alice To Bob
	for {
		n, err := reader.Read(buf)
		if n > 0 {
            log.Println("openChannelReader: ", string(buf[:n]))
			// index := strings.Index(string(buf[:n]), "{")
			// err := json.Unmarshal(buf[index:n], &info)
			// if err != nil {
			// 	log.Fatalln("json.Unmarshal: ", err)
			// }
		}
		if err != nil {
			break
		}
	}

	_, _, err = btcdC.Exec(ctx, []string{"bitcoin-cli", "-regtest", "-rpcuser=rpcuser", "-rpcpassword=rpcpassword", "-generate", "10"})

	if err != nil {
		t.Fatalf("could not create blocks  %+v", err)
	}

    // Get info of bob
    _, getInfoBobReaderTwo, err := LndBobC.Exec(ctx, []string{"lncli", "--tlscertpath", "/home/lnd/.lnd/tls.cert", "--macaroonpath", "home/lnd/.lnd/data/chain/bitcoin/regtest/admin.macaroon", "getinfo"})
	if err != nil {
		t.Errorf("could not get nodeInfo  %+v ", err)
	}


    reader = io.Reader(getInfoBobReaderTwo)
    buf = make([]byte, 3024)


	var bobInfoTwo NodeInfo
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			index := strings.Index(string(buf[:n]), "{")
			err := json.Unmarshal(buf[index:n], &bobInfoTwo)
			if err != nil {
				log.Fatalln("json.Unmarshal: ", err)
			}
             log.Printf("bobInfoTwo: %+v \n\n", bobInfoTwo)
		}
		if err != nil {
			break
		}
	}
	// connect mint to Alice
    macaroon, err := extractInternalFile(ctx, lndAliceC, "/home/lnd/.lnd/data/chain/bitcoin/regtest/admin.macaroon")

    macaroonHex := hex.EncodeToString([]byte(macaroon))

    if err != nil {
        t.Fatalf("could not extract macaroon %+v", err)
    }

    tlsCert, err := extractInternalFile(ctx, lndAliceC, "/home/lnd/.lnd/tls.cert")

    if err != nil {
        t.Fatalf("could not extract tls %+v", err)
    }

    lndAliceIp, err := lndAliceC.ContainerIP(ctx)

    if err != nil {
        t.Fatalf("could not get lndAliceC.ContainerIP %+v", err)
    }

    err = os.Setenv(LND_HOST, lndAliceIp+":" + "10009")
    err = os.Setenv(LND_TLS_CERT, tlsCert)
    err = os.Setenv(LND_MACAROON, macaroonHex)

    if err != nil {
        t.Fatalf("could not set env %+v", err)
    }
    

    lightingComms, err := SetupLightingComms()

    if err != nil {
        t.Fatalf("could not setup lighting comms %+v", err)
    }

    _, err = lightingComms.RequestInvoice(1000)
    if err != nil {
        t.Fatalf("could not setup lighting comms %+v", err)
    }





}

func extractInternalFileInHex(ctx context.Context, container testcontainers.Container, path string) (string, error) {
	_, catData, err := container.Exec(ctx, []string{"cat", path})

	if err != nil {
		return "", err
	}

	reader := io.Reader(catData)
	buf := make([]byte, 4096)

	var data string

    
	for {
		n, err := reader.Read(buf)
		if n > 0 {
            log.Println("buf: ", string(buf[:n]))
			data = hex.EncodeToString(buf[:n])
		}
		if err != nil {
			break
		}
	}

	return data, nil
}
func extractInternalFile(ctx context.Context, container testcontainers.Container, path string) (string, error) {
	catData, err := container.CopyFileFromContainer(ctx,  path)

	if err != nil {
		return "", err
	}

	reader := io.Reader(catData)
	buf := make([]byte, 1024)

	var data string

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			data = string(buf[:n])
		}
		if err != nil {
			break
		}
	}

	return data, nil
}

func readDataFromReader(reader io.Reader) (string, error) {
	buf := make([]byte, 1024)

	var data string

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			fmt.Print(string(buf[:n]))
		}
		if err != nil {
			break
		}
	}

	return data, nil
}

// returns alice, bob, mint
func setUpLightingNetworkEnviroment() (testcontainers.Container,testcontainers.Container, testcontainers.Container, error) {
	// setup

	ctx := context.Background()

	net, err := network.New(ctx,
		network.WithCheckDuplicate(),
		network.WithAttachable(),
		// Makes the network internal only, meaning the host machine cannot access it.
		// Remove or use `network.WithDriver("bridge")` to change the network's mode.
		// network.WithInternal(),
		network.WithDriver("bridge"),
		// network.WithLabels(map[string]string{"this-is-a-test": "value"}),
	)

	if err != nil {
		log.Fatalln("Error: ", err)
		return nil, nil, nil, err
	}

	// Create bitcoind regtest node
	reqbtcd := testcontainers.ContainerRequest{
		Image:        "polarlightning/bitcoind:26.0",
		Name:         "bitcoindbackend",
		WaitingFor:   wait.ForLog("Initialized HTTP server"),
		ExposedPorts: []string{"18443/tcp", "18444/tcp", "28334/tcp", "28335/tcp", "28336/tcp"},
		Networks:     []string{net.Name},

		Cmd: []string{"bitcoind", "-server=1", "-regtest=1", "-rpcuser=rpcuser", "-rpcpassword=rpcpassword", "-debug=1", "-zmqpubrawblock=tcp://0.0.0.0:28334", "-zmqpubrawtx=tcp://0.0.0.0:28335", "-zmqpubhashblock=tcp://0.0.0.0:28336", "-txindex=1", "-dnsseed=0", "-upnp=0", "-rpcbind=0.0.0.0", "-rpcallowip=0.0.0.0/0", "-rpcport=18443", "-rest", "-listen=1", "-listenonion=0", "-fallbackfee=0.0002", "-blockfilterindex=1", "-peerblockfilters=1"},
	}

	btcdC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: reqbtcd,
		Started:          true,
	})

	if err != nil {
	 return nil, nil, nil , fmt.Errorf("could not setup bitcoind %+v", err)
	}

	btcdIP, err := btcdC.ContainerIP(ctx)

	if err != nil {
        return nil, nil, nil, fmt.Errorf("could not get ContainerIP %+v", err)
	}


    log.Println("btcdIP: ", btcdIP)

	_, _, err = btcdC.Exec(ctx, []string{"bitcoin-cli", "-regtest", "-rpcuser=rpcuser", "-rpcpassword=rpcpassword", "createwallet", "wallet"})
	if err != nil {
        return nil, nil, nil, fmt.Errorf("could not create wallet  %+v", err)
	}

	_, _, err = btcdC.Exec(ctx, []string{"bitcoin-cli", "-regtest", "-rpcuser=rpcuser", "-rpcpassword=rpcpassword", "-generate", "101"})

	if err != nil {
        return nil, nil, nil, fmt.Errorf("could not create blocks  %+v", err)
	}

	// create Alice node LND
	reqlndAlice := testcontainers.ContainerRequest{
		Image:        "polarlightning/lnd:0.17.5-beta",
		WaitingFor:   wait.ForLog("Server listening on"),
		ExposedPorts: []string{"18445/tcp", "10009/tcp"},

		Networks: []string{net.Name},
		Cmd:      []string{"lnd", "--noseedbackup", "--trickledelay=5000", "--alias=alice" /* "--externalip=alice", */, "--tlsextradomain=alice", "--tlsextradomain=host.docker.bridge", "--tlsextradomain=host.docker.internal", "--listen=0.0.0.0:9735", "--rpclisten=0.0.0.0:10009", "--restlisten=0.0.0.0:8080", "--bitcoin.active", "--bitcoin.regtest", "--bitcoin.node=bitcoind", "--bitcoind.rpchost=" + btcdIP, "--bitcoind.rpcuser=rpcuser", "--bitcoind.rpcpass=rpcpassword", "--bitcoind.zmqpubrawblock=tcp://" + btcdIP + ":28334", "--bitcoind.zmqpubrawtx=tcp://" + btcdIP + ":28335"},
	}

	lndAliceC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: reqlndAlice,
		Started:          true,
	})



	if err != nil {
        return nil, nil, nil, fmt.Errorf("could not create Alice lnd container  %+v", err)
	}

	_, addressReader, err := lndAliceC.Exec(ctx, []string{"lncli", "--tlscertpath", "/home/lnd/.lnd/tls.cert", "--macaroonpath", "home/lnd/.lnd/data/chain/bitcoin/regtest/admin.macaroon", "newaddress", "p2tr"})

	reader := io.Reader(addressReader)
	buf := make([]byte, 1024)

	type LndAddress struct {
		Address string
	}

	var address LndAddress
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			index := strings.Index(string(buf[:n]), "{")
			err := json.Unmarshal(buf[index:n], &address)
			if err != nil {
				log.Fatalln("json.Unmarshal: ", err)
			}

			fmt.Printf("adress: %+v ", address)
		}
		if err != nil {
			break
		}
	}
    

	// fund Alice node
	_, _, err = btcdC.Exec(ctx, []string{"bitcoin-cli", "-regtest", "-rpcuser=rpcuser", "-rpcpassword=rpcpassword", "sendtoaddress", address.Address, "10"})

	if err != nil {
        return nil, nil, nil, fmt.Errorf("could not fund Alice's wallet  %+v", err)
	}


	_, _, err = btcdC.Exec(ctx, []string{"bitcoin-cli", "-regtest", "-rpcuser=rpcuser", "-rpcpassword=rpcpassword", "-generate", "5"})

	if err != nil {
        return nil, nil, nil, fmt.Errorf("could not create blocks  %+v", err)
	}

	_, _, err = lndAliceC.Exec(ctx, []string{"lncli", "--tlscertpath", "/home/lnd/.lnd/tls.cert", "--macaroonpath", "home/lnd/.lnd/data/chain/bitcoin/regtest/admin.macaroon", "listunspent"})

	if err != nil {
        return nil, nil, nil, fmt.Errorf("could not check balance  %+v ", err)
	}


	// create bob node LND

	reqLndBob := testcontainers.ContainerRequest{
		Image:        "polarlightning/lnd:0.17.5-beta",
		WaitingFor:   wait.ForLog("Server listening on"),
		ExposedPorts: []string{"18446/tcp", "9736/tcp", "10009/tcp", "8080/tcp"},

		Networks: []string{net.Name},
		Cmd:      []string{"lnd", "--noseedbackup", "--trickledelay=5000", "--alias=bob" /* "--externalip=alice", */, "--tlsextradomain=bob", "--tlsextradomain=host.docker.bridge", "--tlsextradomain=host.docker.internal", "--listen=0.0.0.0:9736", "--rpclisten=0.0.0.0:10009", "--restlisten=0.0.0.0:8080", "--bitcoin.active", "--bitcoin.regtest", "--bitcoin.node=bitcoind", "--bitcoind.rpchost=" + btcdIP, "--bitcoind.rpcuser=rpcuser", "--bitcoind.rpcpass=rpcpassword", "--bitcoind.zmqpubrawblock=tcp://" + btcdIP + ":28334", "--bitcoind.zmqpubrawtx=tcp://" + btcdIP + ":28335"},
	}

	LndBobC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: reqLndBob,
		Started:          true,
	})

	lndBobIp, err := LndBobC.ContainerIP(ctx)

	if err != nil {
        return nil, nil, nil, fmt.Errorf("could not get lndAliceC.ContainerIP %+v", err)
	}


    _, getInfoBobReader, err := LndBobC.Exec(ctx, []string{"lncli", "--tlscertpath", "/home/lnd/.lnd/tls.cert", "--macaroonpath", "home/lnd/.lnd/data/chain/bitcoin/regtest/admin.macaroon", "getinfo"})
	if err != nil {
        return nil, nil, nil, fmt.Errorf("could not get nodeInfo  %+v ", err)
	}


    reader = io.Reader(getInfoBobReader)
    buf = make([]byte, 3024)

    type NodeInfo struct {
        IdentityPubkey string `json:"identity_pubkey"`
        NumPeers int `json:"num_peers"`
        NumPendingChannels int `json:"num_pending_channels"`
        NumActiveChannels int `json:"num_active_channels"`
        NumInactiveChannels int `json:"num_inactive_channels"`
    }

	var bobInfo NodeInfo
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			index := strings.Index(string(buf[:n]), "{")
			err := json.Unmarshal(buf[index:n], &bobInfo)
			if err != nil {
				log.Fatalln("json.Unmarshal: ", err)
			}
		}
		if err != nil {
			break
		}
	}

	// open channel between Alice and Bob
    connectionStr := bobInfo.IdentityPubkey + "@" + lndBobIp + ":" + "9736"
    _, connectPeerReader, err := lndAliceC.Exec(ctx, []string{"lncli", "--tlscertpath", "/home/lnd/.lnd/tls.cert", "--macaroonpath", "home/lnd/.lnd/data/chain/bitcoin/regtest/admin.macaroon", "connect",connectionStr })

	if err != nil {
        return nil, nil, nil, fmt.Errorf("could not get nodeInfo  %+v ", err)
	}
    reader = io.Reader(connectPeerReader)
    buf = make([]byte, 1024)


    // connect Alice To Bob
	for {
		n, err := reader.Read(buf)
		if n > 0 {
            log.Println("connectPeerReader: ", string(buf[:n]))
			// index := strings.Index(string(buf[:n]), "{")
			// err := json.Unmarshal(buf[index:n], &info)
			// if err != nil {
			// 	log.Fatalln("json.Unmarshal: ", err)
			// }
		}
		if err != nil {
			break
		}
	}
	// open channel between Alice and Bob
    // connectionStr := bobInfo.IdentityPubkey + "@" + lndBobIp + ":" + "9736"
    _, openChannelReader, err := lndAliceC.Exec(ctx, []string{"lncli", "--tlscertpath", "/home/lnd/.lnd/tls.cert", "--macaroonpath", "home/lnd/.lnd/data/chain/bitcoin/regtest/admin.macaroon", "openchannel", "--node_key",  bobInfo.IdentityPubkey, "--fundmax" })

	if err != nil {
        return nil, nil, nil, fmt.Errorf("could not get nodeInfo  %+v ", err)
	}
    reader = io.Reader(openChannelReader)
    buf = make([]byte, 1024)


    // connect Alice To Bob
	for {
		n, err := reader.Read(buf)
		if n > 0 {
            log.Println("openChannelReader: ", string(buf[:n]))
			// index := strings.Index(string(buf[:n]), "{")
			// err := json.Unmarshal(buf[index:n], &info)
			// if err != nil {
			// 	log.Fatalln("json.Unmarshal: ", err)
			// }
		}
		if err != nil {
			break
		}
	}

	_, _, err = btcdC.Exec(ctx, []string{"bitcoin-cli", "-regtest", "-rpcuser=rpcuser", "-rpcpassword=rpcpassword", "-generate", "10"})

	if err != nil {
        return nil, nil, nil, fmt.Errorf("could not create blocks  %+v", err)
	}

    // Get info of bob
    _, getInfoBobReaderTwo, err := LndBobC.Exec(ctx, []string{"lncli", "--tlscertpath", "/home/lnd/.lnd/tls.cert", "--macaroonpath", "home/lnd/.lnd/data/chain/bitcoin/regtest/admin.macaroon", "getinfo"})
	if err != nil {
        return nil, nil, nil, fmt.Errorf("could not get nodeInfo  %+v ", err)
	}


    reader = io.Reader(getInfoBobReaderTwo)
    buf = make([]byte, 3024)


	var bobInfoTwo NodeInfo
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			index := strings.Index(string(buf[:n]), "{")
			err := json.Unmarshal(buf[index:n], &bobInfoTwo)
			if err != nil {
				log.Fatalln("json.Unmarshal: ", err)
			}
             log.Printf("bobInfoTwo: %+v \n\n", bobInfoTwo)
		}
		if err != nil {
			break
		}
	}
	// connect mint to Alice
    macaroon, err := extractInternalFile(ctx, lndAliceC, "/home/lnd/.lnd/data/chain/bitcoin/regtest/admin.macaroon")

    macaroonHex := hex.EncodeToString([]byte(macaroon))

    if err != nil {
        return nil, nil, nil, fmt.Errorf("could not extract macaroon %+v", err)
    }

    tlsCert, err := extractInternalFile(ctx, lndAliceC, "/home/lnd/.lnd/tls.cert")

    if err != nil {
        return nil, nil, nil, fmt.Errorf("could not extract tls %+v", err)
    }

    lndAliceIp, err := lndAliceC.ContainerIP(ctx)

    if err != nil {
        return nil, nil, nil, fmt.Errorf("could not get lndAliceC.ContainerIP %+v", err)
    }

    err = os.Setenv(LND_HOST, lndAliceIp+":" + "10009")
    err = os.Setenv(LND_TLS_CERT, tlsCert)
    err = os.Setenv(LND_MACAROON, macaroonHex)

    if err != nil {
        return nil, nil, nil, fmt.Errorf("could not set env %+v", err)
    }
    


    // return alice, bob, mint
    return lndAliceC, LndBobC, btcdC, nil

}
