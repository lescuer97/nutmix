package comms

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

// This is used for testing purpose
// returns alice, bob, btcNode, aliceLnBits, error
func SetUpLightingNetworkTestEnviroment(ctx context.Context, names string) (testcontainers.Container, testcontainers.Container, testcontainers.Container, testcontainers.Container, error) {
	// setup
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
		return nil, nil, nil, nil, err
	}

	// Create bitcoind regtest node
	reqbtcd := testcontainers.ContainerRequest{
		Image:        "polarlightning/bitcoind:26.0",
		Name:         "bitcoindbackend" + names,
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
		return nil, nil, nil, nil, fmt.Errorf("could not setup bitcoind %w", err)
	}

	btcdIP, err := btcdC.ContainerIP(ctx)

	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not get ContainerIP %w", err)
	}

	_, _, err = btcdC.Exec(ctx, []string{"bitcoin-cli", "-regtest", "-rpcuser=rpcuser", "-rpcpassword=rpcpassword", "createwallet", "wallet"})
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not create wallet  %w", err)
	}

	_, _, err = btcdC.Exec(ctx, []string{"bitcoin-cli", "-regtest", "-rpcuser=rpcuser", "-rpcpassword=rpcpassword", "-generate", "101"})

	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not create blocks  %w", err)
	}

	// create Alice node LND
	reqlndAlice := testcontainers.ContainerRequest{
		Image:        "polarlightning/lnd:0.17.5-beta",
		WaitingFor:   wait.ForLog("Server listening on"),
		ExposedPorts: []string{"18445/tcp", "10009/tcp", "8080/tcp", "9735/tcp"},
		Name:         "lndAlice" + names,

		Networks: []string{net.Name},
		Cmd:      []string{"lnd", "--noseedbackup", "--trickledelay=5000", "--alias=alice" /* "--externalip=alice", */, "--tlsextradomain=alice", "--tlsextradomain=host.docker.bridge", "--tlsextradomain=host.docker.internal", "--listen=0.0.0.0:9735", "--rpclisten=0.0.0.0:10009", "--restlisten=0.0.0.0:8080", "--bitcoin.active", "--bitcoin.regtest", "--bitcoin.node=bitcoind", "--bitcoind.rpchost=" + btcdIP, "--bitcoind.rpcuser=rpcuser", "--bitcoind.rpcpass=rpcpassword", "--bitcoind.zmqpubrawblock=tcp://" + btcdIP + ":28334", "--bitcoind.zmqpubrawtx=tcp://" + btcdIP + ":28335"},
	}

	lndAliceC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: reqlndAlice,
		Started:          true,
	})

	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not create Alice lnd container  %w", err)
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

		}
		if err != nil {
			break
		}
	}

	// fund Alice node
	_, _, err = btcdC.Exec(ctx, []string{"bitcoin-cli", "-regtest", "-rpcuser=rpcuser", "-rpcpassword=rpcpassword", "sendtoaddress", address.Address, "10"})

	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not fund Alice's wallet  %w", err)
	}

	_, _, err = btcdC.Exec(ctx, []string{"bitcoin-cli", "-regtest", "-rpcuser=rpcuser", "-rpcpassword=rpcpassword", "-generate", "10"})

	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not create blocks  %w", err)
	}

	_, _, err = lndAliceC.Exec(ctx, []string{"lncli", "--tlscertpath", "/home/lnd/.lnd/tls.cert", "--macaroonpath", "home/lnd/.lnd/data/chain/bitcoin/regtest/admin.macaroon", "listunspent"})

	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not check balance  %w ", err)
	}

	// create bob node LND

	reqLndBob := testcontainers.ContainerRequest{
		Image:        "polarlightning/lnd:0.17.5-beta",
		WaitingFor:   wait.ForLog("Server listening on"),
		ExposedPorts: []string{"18446/tcp", "9736/tcp", "10009/tcp", "8081/tcp"},
		Name:         "lndBob" + names,

		Networks: []string{net.Name},
		Cmd:      []string{"lnd", "--noseedbackup", "--trickledelay=5000", "--alias=bob" /* "--externalip=alice", */, "--tlsextradomain=bob", "--tlsextradomain=host.docker.bridge", "--tlsextradomain=host.docker.internal", "--listen=0.0.0.0:9736", "--rpclisten=0.0.0.0:10009", "--restlisten=0.0.0.0:8081", "--bitcoin.active", "--bitcoin.regtest", "--bitcoin.node=bitcoind", "--bitcoind.rpchost=" + btcdIP, "--bitcoind.rpcuser=rpcuser", "--bitcoind.rpcpass=rpcpassword", "--bitcoind.zmqpubrawblock=tcp://" + btcdIP + ":28334", "--bitcoind.zmqpubrawtx=tcp://" + btcdIP + ":28335"},
	}

	LndBobC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: reqLndBob,
		Started:          true,
	})

	lndBobIp, err := LndBobC.ContainerIP(ctx)

	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not get lndAliceC.ContainerIP %w", err)
	}

	_, getInfoBobReader, err := LndBobC.Exec(ctx, []string{"lncli", "--tlscertpath", "/home/lnd/.lnd/tls.cert", "--macaroonpath", "home/lnd/.lnd/data/chain/bitcoin/regtest/admin.macaroon", "getinfo"})
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not get nodeInfo  %w ", err)
	}

	reader = io.Reader(getInfoBobReader)
	buf = make([]byte, 3024)

	type NodeInfo struct {
		IdentityPubkey      string `json:"identity_pubkey"`
		NumPeers            int    `json:"num_peers"`
		NumPendingChannels  int    `json:"num_pending_channels"`
		NumActiveChannels   int    `json:"num_active_channels"`
		NumInactiveChannels int    `json:"num_inactive_channels"`
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

	// peer connect between Alice and Bob
	connectionStr := bobInfo.IdentityPubkey + "@" + lndBobIp + ":" + "9736"
	_, _, err = lndAliceC.Exec(ctx, []string{"lncli", "--tlscertpath", "/home/lnd/.lnd/tls.cert", "--macaroonpath", "home/lnd/.lnd/data/chain/bitcoin/regtest/admin.macaroon", "connect", connectionStr})

	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not get nodeInfo  %w ", err)
	}

	// open channel between Alice and Bob
	_, _, err = lndAliceC.Exec(ctx, []string{"lncli", "--tlscertpath", "/home/lnd/.lnd/tls.cert", "--macaroonpath", "home/lnd/.lnd/data/chain/bitcoin/regtest/admin.macaroon", "openchannel", "--node_key", bobInfo.IdentityPubkey, "--fundmax", "--push_amt", "10000000"})

	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not get nodeInfo  %w ", err)
	}

	_, _, err = btcdC.Exec(ctx, []string{"bitcoin-cli", "-regtest", "-rpcuser=rpcuser", "-rpcpassword=rpcpassword", "-generate", "50"})

	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not create blocks  %w", err)
	}

	// Get info of bob
	_, getInfoBobReaderTwo, err := LndBobC.Exec(ctx, []string{"lncli", "--tlscertpath", "/home/lnd/.lnd/tls.cert", "--macaroonpath", "home/lnd/.lnd/data/chain/bitcoin/regtest/admin.macaroon", "getinfo"})
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not get nodeInfo  %w ", err)
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
		}
		if err != nil {
			break
		}
	}

	if bobInfoTwo.NumActiveChannels == 0 {
		return nil, nil, nil, nil, fmt.Errorf("could not open channel  %w", err)
	}
	// connect mint to Alice
	macaroon, err := ExtractInternalFile(ctx, lndAliceC, "/home/lnd/.lnd/data/chain/bitcoin/regtest/admin.macaroon")

	macaroonHex := hex.EncodeToString([]byte(macaroon))

	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not extract macaroon %w", err)
	}

	tlsCert, err := ExtractInternalFile(ctx, lndAliceC, "/home/lnd/.lnd/tls.cert")

	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not extract tls %w", err)
	}

	lndAliceIp, err := lndAliceC.ContainerIP(ctx)

	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not get lndAliceC.ContainerIP %w", err)
	}

	alicePort := "10009"

	tlsCertPath := "/.lnd/tls.cert"

	err = os.Setenv(LND_HOST, lndAliceIp+":"+alicePort)
	err = os.Setenv(LND_TLS_CERT, tlsCert)
	err = os.Setenv(LND_MACAROON, macaroonHex)

	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not set env %w", err)
	}

	aliceLnbitsEnvVariables := make(map[string]string)

	aliceLnbitsEnvVariables["LNBITS_BACKEND_WALLET_CLASS"] = "LndWallet"
	aliceLnbitsEnvVariables["LND_GRPC_ENDPOINT"] = lndAliceIp
	aliceLnbitsEnvVariables["LND_GRPC_PORT"] = alicePort
	aliceLnbitsEnvVariables["LND_GRPC_CERT"] = tlsCertPath
	aliceLnbitsEnvVariables["LND_GRPC_MACAROON"] = macaroonHex
	aliceLnbitsEnvVariables["LNBITS_ADMIN_UI"] = "true"

	tlscertReader := strings.NewReader(tlsCert)

	aliceLnbitsContainerReq := testcontainers.ContainerRequest{
		Image:      "lnbits/lnbits",
		WaitingFor: wait.ForLog("Application startup complete"),
		Files: []testcontainers.ContainerFile{
			{
				Reader:            tlscertReader,
				ContainerFilePath: tlsCertPath,
				FileMode:          0o700,
			},
		},
		ExposedPorts: []string{"5000/tcp"},
		Name:         "aliceLNBITS" + names,
		Env:          aliceLnbitsEnvVariables,
		Networks:     []string{net.Name},
	}

	aliceLnbitsC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: aliceLnbitsContainerReq,
		Started:          true,
	})

	aliceLnbitsC.CopyToContainer(ctx, []byte(tlsCert), tlsCertPath, 0o700)

	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not get aliceLnbitsC %w", err)
	}

	aliceLnbitsIp, err := aliceLnbitsC.ContainerIP(ctx)

	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not get aliceLnbitsC.ContainerIP %w", err)
	}

	// Get API key for aliceLnbits

	// make request for first install
	client := &http.Client{}

	firstInstallBody := struct {
		Username       string `json:"username"`
		Password       string `json:"password"`
		PasswordRepeat string `json:"password_repeat"`
	}{
		Username:       "admin",
		Password:       "password",
		PasswordRepeat: "password",
	}
	jsonBytes, err := json.Marshal(firstInstallBody)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("json.Marshal: %w", err)
	}

	b := bytes.NewBuffer(jsonBytes)

	req, err := http.NewRequest("PUT", "http://"+aliceLnbitsIp+":5000/api/v1/auth/first_install", b)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not make request %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not make request %w", err)
	}

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("ioutil.ReadAll: %w", err)
	}
	var response struct {
		AccessToken string `json:"access_token"`
	}

	err = json.Unmarshal(body, &response)

	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("json.Unmarshal: %w", err)
	}
	fmt.Printf("Body: %+v ", response)

	// get auth settings
	authBody := struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}{
		Username: "admin",
		Password: "password",
	}

	jsonBytes, err = json.Marshal(authBody)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("json.Marshal: %w", err)
	}

	b = bytes.NewBuffer(jsonBytes)

	walletsRequest, err := http.NewRequest("GET", "http://"+aliceLnbitsIp+":5000/api/v1/wallets", nil)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not make request %w", err)
	}

	walletsRequest.Header.Add("Authorization", "Bearer "+response.AccessToken)
	walletsRequest.Header.Add("cookie_access_token", response.AccessToken)

	respWallet, err := client.Do(walletsRequest)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not make response %w", err)
	}

	body, err = io.ReadAll(respWallet.Body)

	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("ioutil.ReadAll: %w", err)
	}
	var responseWallet []struct {
		AdminKey string `json:"adminkey"`
	}

	err = json.Unmarshal(body, &responseWallet)

	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("json.Unmarshal: %w", err)
	}

	if len(responseWallet) == 0 {
		return nil, nil, nil, nil, fmt.Errorf("no wallet found")
	}

	fmt.Printf("AdminKey: %+v ", responseWallet[0].AdminKey)
	err = os.Setenv(MINT_LNBITS_KEY, responseWallet[0].AdminKey)
	err = os.Setenv(MINT_LNBITS_ENDPOINT, "http://"+aliceLnbitsIp+":5000")
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not set env %w", err)
	}

	fmt.Println("Response: ", string(body))

	// generate wallet

	// return alice, bob, btcNode, aliceLnbits, error
	return lndAliceC, LndBobC, btcdC, aliceLnbitsC, nil

}
func ExtractInternalFile(ctx context.Context, container testcontainers.Container, path string) (string, error) {
	catData, err := container.CopyFileFromContainer(ctx, path)

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

func ReadDataFromReader(reader io.Reader) (string, error) {
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
