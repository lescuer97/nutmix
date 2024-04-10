_Disclaimer: The author is NOT a cryptographer and this work has not been reviewed. This means that there is very likely a fatal flaw somewhere. Cashu is still experimental and not production-ready._

_Don't be reckless:_ This project is in early development; it does, however, work with real sats! Always use amounts you don't mind losing.

_This project is in active development; it has not gone through optimization and refactoring yet_.

# cashu-v4v

This has now been turned into an attempt at a Golang reference implementation of the Cashu Protocol and a SvelteKit wallet for it. Initially, this was a project to create a value-for-value wallet with Cashu at its core.

## Purpose of this project

This is just an attempt to learn Go. I saw the work made by calle on the Cashu protocol and was fascinated by the awesomeness of the project. So I decided to make a reference implementation of the mint in Go. I am attempting to make all the NUTs available as well as some other ideas such as: Monero ECash token.

The wallet is going to be made with SvelteKit (probably not the best option). I will implement the wallet with the possibility of multiple denominations of eCash (SATs, Monero, eUSD). I would also like to have some Nostr functionality.

## Supported NUTs

Implemented [NUTs](https://github.com/cashubtc/nuts/):

- [x] [NUT-00](https://github.com/cashubtc/nuts/blob/main/00.md)
- [x] [NUT-01](https://github.com/cashubtc/nuts/blob/main/01.md)
- [x] [NUT-02](https://github.com/cashubtc/nuts/blob/main/02.md)
- [] [NUT-03](https://github.com/cashubtc/nuts/blob/main/03.md)
- [] [NUT-04](https://github.com/cashubtc/nuts/blob/main/04.md)
- [] [NUT-05](https://github.com/cashubtc/nuts/blob/main/05.md)
- [x] [NUT-06](https://github.com/cashubtc/nuts/blob/main/06.md)
- [ ] [NUT-07](https://github.com/cashubtc/nuts/blob/main/07.md)
- [ ] [NUT-08](https://github.com/cashubtc/nuts/blob/main/08.md)
- [ ] [NUT-10](https://github.com/cashubtc/nuts/blob/main/10.md)
- [ ] [NUT-11](https://github.com/cashubtc/nuts/blob/main/11.md)
- [ ] [NUT-12](https://github.com/cashubtc/nuts/blob/main/12.md)
- [ ] [NUT-13](https://github.com/cashubtc/nuts/blob/main/13.md)



## Development

Right now, the wallet and the mint are being developed as completely separate projects.


### Mint

TODO

#### Development of mint


#### Run PostgreSQL DB

### Wallet
TODO

