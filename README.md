_Disclaimer: The author is NOT a cryptographer and this work has not been reviewed. This means that there is very likely a fatal flaw somewhere. Cashu is still experimental and not production-ready._

_Don't be reckless:_ This project is in early development; it does, however, work with real sats! Always use amounts you don't mind losing.

_This project is in active development; it has not gone through optimization and refactoring yet_.

# Nutmix

This has now been turned into a Golang reference implementation of the Cashu Protocol and a SvelteKit wallet for it. This was originally a project to create a value-for-value wallet with Cashu at its core (Might come in the future).

## Purpose of this project

This is an attempt to learn Go and eCash. I saw the work made by calle on the Cashu protocol and was fascinated by the awesomeness of the project. So I decided to make a reference implementation of the mint in Go. 
I am attempting to make all the NUTs available as well as some other ideas such as: Monero ECash token.

The wallet is going to be made with SvelteKit (probably not the best option). I will implement the wallet with the possibility of multiple denominations of eCash (SATs, Monero, eUSD). I would also like to have some Nostr functionality.

## Supported NUTs

Implemented [NUTs](https://github.com/cashubtc/nuts/):

- [x] [NUT-00](https://github.com/cashubtc/nuts/blob/main/00.md)
- [x] [NUT-01](https://github.com/cashubtc/nuts/blob/main/01.md)
- [x] [NUT-02](https://github.com/cashubtc/nuts/blob/main/02.md)
- [x] [NUT-03](https://github.com/cashubtc/nuts/blob/main/03.md)
- [x] [NUT-04](https://github.com/cashubtc/nuts/blob/main/04.md)
- [x] [NUT-05](https://github.com/cashubtc/nuts/blob/main/05.md)
- [x] [NUT-06](https://github.com/cashubtc/nuts/blob/main/06.md)
- [x] [NUT-07](https://github.com/cashubtc/nuts/blob/main/07.md)
- [ ] [NUT-08](https://github.com/cashubtc/nuts/blob/main/08.md)
- [ ] [NUT-10](https://github.com/cashubtc/nuts/blob/main/10.md)
- [ ] [NUT-11](https://github.com/cashubtc/nuts/blob/main/11.md)
- [ ] [NUT-12](https://github.com/cashubtc/nuts/blob/main/12.md)
- [ ] [NUT-13](https://github.com/cashubtc/nuts/blob/main/13.md)
- [ ] [NUT-14](https://github.com/cashubtc/nuts/blob/main/14.md)

## Development

Right now, the wallet and the mint are being developed as completely separate projects.

### Things to implement

- [x] Implement obligatory NUTs.
- [x] Add Tests for crypto functions.
- [ ] Add Tests for general functionality of mint.
- [ ] Make GUI for data about the mint and actions.
- [ ] Explore the use of http std library and not Gin.


### Run the Mint

This project is thought to be able to be ran on a docker container or locally.

if you want to run the project in docker you will need two thinks. 

You'll need  the correct variables in an `.env` file. Use the env.example file as reference.

You need to make sure to use a strong `POSTGRES_PASSWORD` and make user the username and password are the same in the `DATABASE_URL`

It's important to set up the variables under `LIGHTNING CONNECTION` for connecting to a node. 

if you want to run with traefik and you also need to fill variables below `HOSTING` for your domains.

If you have this correctly setup it should be as easy as running a simple docker command on linux:

```
docker compose up 
```

### Development

#### Mint


if you want to develop for the project I recommend personnaly run a hybrid setup. I run the mint locally and the db on docker. 

I have a special development docker compose called: `docker-compose-dev.yml`. This is for simpler development without having traefik in the middle.

1. run the database in docker. please check you have the necesary info in the `.env` file. 

```
docker compose -f docker-compose-dev.yml up db
```

2. Run the mint locally. 

```
# build the project
go run cmd/nutmix/*.go
```

 

### Wallet
TODO!

