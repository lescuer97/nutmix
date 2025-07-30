_Disclaimer: The author is NOT a cryptographer and this work has not been reviewed. This means that there is very likely
a fatal flaw somewhere. Cashu is still experimental and not production-ready._

_Don't be reckless:_ This project is in early development; it does, however, work with real sats! Always use amounts you
don't mind losing.

_This project is in active development; it has not gone through optimization and refactoring yet_.

# Nutmix

Cashu protocol mint focused on ease of use and feature completeness.

Please test in Mutinynet at: *https://mutinynet.nutmix.cash*

This is an alternative Cashu mint written in go. It's specifically just a mint with the objective to minimize the code
and complexity.  
## Objective
The mint includes it's proper dashboard to manage administration and logs. 

### Run the Mint

This project is thought to be able to be ran on a docker container or locally.

Here is what you need to know to change to run nutmix in docker:
*Most of the setup process will happen inside the Admin dashboard.* 

You'll need  the correct variables in an `.env` file first. Use the env.example file as reference.

- You need to make sure to use a strong `POSTGRES_PASSWORD` and make user the username and password are the same in the
`DATABASE_URL`

- Add private key using the `MINT_PRIVATE_KEY` enviroment variable or pick connect to a remote signer. 

- To login into the admin dashboard and change the rest of settings add your npub to `ADMIN_NOSTR_NPUB` enviroment variable. 

The mint will stop and Print out what you are missing if you don't have this 4 Items setup.


### Running docker 
In case you want to run the docker compose file using traefik and you also need to fill variables below `HOSTING` for your domains.
If you have this correctly setup it should be as easy as running a simple docker command on linux:

```bash 
docker compose up -d 
```

## Setting up a remote signer.

Right now there are two remote signer implementations. 

- [Nutvault](https://github.com/lescuer97/nutvault)
- [cdk-signatory](https://github.com/cashubtc/cdk/tree/main/crates/cdk-signatory)

There is a new enviroment variable called `SIGNER_TYPE`. If you want to use the remote signer you need to set the
options `abstract_socket` or `network`. This will then will look for the signer to connect.  If you pick network you
will also need to set the `NETWORK_SIGNER_ADDRESS` env variable.

### Setup mTLS for signer
The mint communicates with the remote signer using mTLS.
You will need to set environment variables for signaling the routes for mTLS.

```bash
SIGNER_CLIENT_TLS_KEY=<route to file>
SIGNER_CLIENT_TLS_CERT=<route to file>
SIGNER_CA_CERT=<route to file>
```

#### Video on .env setup
[![Video on .env setup](https://cdn.hzrd149.com/0930b6e46cfe03a70345930d55b2eff51b0eb39d6e6eb4305b42b7736398f49c.png)](https://cdn.hzrd149.com/0ef3cb33401dbdd002039d01c0f749491c26720a80b23b885ae0f569ebd9f7b3.mp4)

#### Setup Lightning node
[![Setup Lightning node](https://cdn.hzrd149.com/c2175c7a310026f0450f98146f9dd180979909aaa464aa4376a75eb25b013b10.png)](https://cdn.hzrd149.com/905025ea49d48e36890f87ab05a7be75b141331e25ec8a326a29adfc9cb4cd0a.mp4)

#### Walkthrough of dashboard
[![Walkthrough of dashboard](https://cdn.hzrd149.com/9f967999398e74ffb5ae079bb7e06b58ef8470204b05a21647c5b4e18c71c8d9.png)](https://cdn.hzrd149.com/72a5a65e027370084d45586084098f97ae3631f86bad932656b5c9532be7ba93.mp4)


## Supported NUTs
[NUTs REPO](https://github.com/cashubtc/nuts/):

- [x] [NUT-00](https://github.com/cashubtc/nuts/blob/main/00.md)
- [x] [NUT-01](https://github.com/cashubtc/nuts/blob/main/01.md)
- [x] [NUT-02](https://github.com/cashubtc/nuts/blob/main/02.md)
- [x] [NUT-03](https://github.com/cashubtc/nuts/blob/main/03.md)
- [x] [NUT-04](https://github.com/cashubtc/nuts/blob/main/04.md)
- [x] [NUT-05](https://github.com/cashubtc/nuts/blob/main/05.md)
- [x] [NUT-06](https://github.com/cashubtc/nuts/blob/main/06.md)
- [x] [NUT-07](https://github.com/cashubtc/nuts/blob/main/07.md)
- [x] [NUT-08](https://github.com/cashubtc/nuts/blob/main/08.md)
- [x] [NUT-10](https://github.com/cashubtc/nuts/blob/main/10.md)
- [x] [NUT-11](https://github.com/cashubtc/nuts/blob/main/11.md)
- [x] [NUT-12](https://github.com/cashubtc/nuts/blob/main/12.md)
- [x] [NUT-14](https://github.com/cashubtc/nuts/blob/main/14.md)
- [x] [NUT-15](https://github.com/cashubtc/nuts/blob/main/15.md)
- [x] [NUT-19](https://github.com/cashubtc/nuts/blob/main/19.md)
- [x] [NUT-20](https://github.com/cashubtc/nuts/blob/main/20.md)
- [x] [NUT-21](https://github.com/cashubtc/nuts/blob/main/21.md)
- [x] [NUT-22](https://github.com/cashubtc/nuts/blob/main/22.md)
Non official NUT:
- [x] [NUT-XX](https://github.com/cashubtc/nuts/blob/main/22.md)


## Development

If you want to develop for the project I personally run a hybrid setup. I run the mint locally and the db on docker. 

I have a special development docker compose called: `docker-compose-dev.yml`. This is for simpler development without having traefik in the middle.

1. run the database in docker. Please check you have the necessary info in the `.env` file. 

``` docker compose -f docker-compose-dev.yml up db ```

2. Run the mint locally. 

``` # build the project go run cmd/nutmix/*.go ```

#### Generate remote-signer proto code

```
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative --experimental_allow_proto3_optional internal/gen/signer.proto
```
### Support 

Pull requests and suggestions are always welcomed. The more people have eyes on this the better.

If you can donate monetarily it would be greatly appreciated. The funds would go to the development of the mint and
servers for testing.


*on-chain silent payments*

```
sp1qq0fju879lh2rgvwjjd7e78pg4gnr7a8aumth8qlezdgjs2rwzk7ssq5jm7v27cuuk5dyjfurdy8t8jflkcx0sluwez350kjjd45y7nnx3vgmjqjq
```

*Donate with lightning*

[nutmix@npub.cash](https://npub.cash/pay/nutmix)


*Donate with on-chain*

```
bc1qp7lswgftpgrkt00vszrm63dmkq3nuxjv60czk6
```

*Donate with Monero*

```
84yCRZY6BXebs8xWE6Yzj6S6cE17uLhkTSynneVPmejjWAcgBtnV7UEUiZqJNLE4pXaPmXNkJuhcAYbpu49zAdVsEZqqxac
```


