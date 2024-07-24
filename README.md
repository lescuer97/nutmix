_Disclaimer: The author is NOT a cryptographer and this work has not been reviewed. This means that there is very likely
a fatal flaw somewhere. Cashu is still experimental and not production-ready._

_Don't be reckless:_ This project is in early development; it does, however, work with real sats! Always use amounts you
don't mind losing.

_This project is in active development; it has not gone through optimization and refactoring yet_.

# Nutmix

Cashu protocol mint focused on ease of use and feature completeness.

Please test in Mutinynet at: *https://mutinynet.nutmix.cash*

## Purpose of this project

I saw the work made by calle on the Cashu protocol and was fascinated by the awesomeness of the project. So I decided to make an implementation of the mint in Go. 

I'm also attempting to make all NUTs available as well as some other ideas such as: Monero and FIAT ecash.

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
- [x] [NUT-08](https://github.com/cashubtc/nuts/blob/main/08.md)
- [x] [NUT-10](https://github.com/cashubtc/nuts/blob/main/10.md)
- [x] [NUT-11](https://github.com/cashubtc/nuts/blob/main/11.md)
- [x] [NUT-12](https://github.com/cashubtc/nuts/blob/main/12.md)
- [ ] [NUT-13](https://github.com/cashubtc/nuts/blob/main/13.md)
- [ ] [NUT-14](https://github.com/cashubtc/nuts/blob/main/14.md)

## Objective
Build the fastest and most secure implementation of an ecash mint possible. This would include and admin dashboard to be
able to monitor and control de behaviour of the mint.

## Roadmap
At this moment NUTS are up to P2PK (NUT-11) are implemented and working. I plan to keep going until all NUTS are done.

1. ~~Finish Milestones for [V1](https://github.com/lescuer97/nutmix/milestone/1).~~
2. Finish NUTS until NUT-15. 
3. Add dashboard for controlling aspects of the mint.
    - Nostr Login
    - Rotate keysets.
    - Monitor Mint activity.
    - Emmit blind signatures for certain users.
    - Activate Nostr only mode.
    - Change mint messages
5. Add support for other lightning nodes. Ex: core-lighting, Strike, Greenlight.
4. Add Monero Support this would probably include a way to exchange in between Bitcoin and Monero.
5. Tor only mode
6. Nostr only Mode.
7. Remote signing. This would leave the mint in a highly available server but the lightning transactions and tokens
   would be signed and verified on a secure enclave. This could be something like a hardware device or AWS Nitro. I
   would probably take inspiration or directly use something like [VLS](https://vls.tech/)

### Run the Mint

This project is thought to be able to be ran on a docker container or locally.

if you want to run the project in docker you will need two things.

You'll need  the correct variables in an `.env` file. Use the env.example file as reference.

You need to make sure to use a strong `POSTGRES_PASSWORD` and make user the username and password are the same in the
`DATABASE_URL`

It's important to set up the variables under `LIGHTNING CONNECTION` for connecting to a node. 

Please use a secure Private for deriving keysets.

if you want to run with docker traefik and you also need to fill variables below `HOSTING` for your domains.

If you have this correctly setup it should be as easy as running a simple docker command on linux:

If you missed and important variable for the mint. The mint should panic and let you know.

``` docker compose up -d ```

#### How to rotate a keyset up

key rotation schema:  /keydenomition/version/amount.

In case that you want to rotate the keys from a given keyset, you should set the given seed to active=false in your
Database. Then restart the mint. The mint will automatically catch not having any active seed for a given unit and will rotate the key. 

if you have a key: /sat/1/1 and rotate up it will turn out /sat/2/1

### Development

If you want to develop for the project I personally run a hybrid setup. I run the mint locally and the db on docker. 

I have a special development docker compose called: `docker-compose-dev.yml`. This is for simpler development without having traefik in the middle.

1. run the database in docker. Please check you have the necessary info in the `.env` file. 

``` docker compose -f docker-compose-dev.yml up db ```

2. Run the mint locally. 

``` # build the project go run cmd/nutmix/*.go ```

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


