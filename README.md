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

## Video Walkthrough
#### Video on .env setup
https://github.com/user-attachments/assets/4b626a1f-4107-4ba0-be87-634424e0b565

#### Walkthrough of dashboard
https://github.com/user-attachments/assets/29943daa-9429-40c0-8df5-97ca3b044a77


## Supported NUTs
[NUTs REPO](https://github.com/cashubtc/nuts/):

- [x] [NUT-00](https://github.com/cashubtc/nuts/blob/main/00.md) - Cryptography and Models
- [x] [NUT-01](https://github.com/cashubtc/nuts/blob/main/01.md) - Mint public keys
- [x] [NUT-02](https://github.com/cashubtc/nuts/blob/main/02.md) - Keysets and fees
- [x] [NUT-03](https://github.com/cashubtc/nuts/blob/main/03.md) - Swapping tokens
- [x] [NUT-04](https://github.com/cashubtc/nuts/blob/main/04.md) - Minting tokens
- [x] [NUT-05](https://github.com/cashubtc/nuts/blob/main/05.md) - Melting tokens
- [x] [NUT-06](https://github.com/cashubtc/nuts/blob/main/06.md) - Mint info
- [x] [NUT-07](https://github.com/cashubtc/nuts/blob/main/07.md) - Token state check
- [x] [NUT-08](https://github.com/cashubtc/nuts/blob/main/08.md) - Overpaid Lightning fees
- [x] [NUT-09](https://github.com/cashubtc/nuts/blob/main/09.md) - Signature restore
- [x] [NUT-10](https://github.com/cashubtc/nuts/blob/main/10.md) - Spending conditions
- [x] [NUT-11](https://github.com/cashubtc/nuts/blob/main/11.md) - Pay-To-Pubkey (P2PK)
- [x] [NUT-12](https://github.com/cashubtc/nuts/blob/main/12.md) - DLEQ proofs
- [x] [NUT-14](https://github.com/cashubtc/nuts/blob/main/14.md) - Hashed Timelock Contracts (HTLCs)
- [x] [NUT-15](https://github.com/cashubtc/nuts/blob/main/15.md) - Partial multi-path payments (MPP)
- [x] [NUT-17](https://github.com/cashubtc/nuts/blob/main/17.md) - WebSocket subscriptions
- [x] [NUT-19](https://github.com/cashubtc/nuts/blob/main/19.md) - Cached Responses
- [x] [NUT-20](https://github.com/cashubtc/nuts/blob/main/20.md) - Signature on Mint Quote
- [x] [NUT-21](https://github.com/cashubtc/nuts/blob/main/21.md) - Clear authentication
- [x] [NUT-22](https://github.com/cashubtc/nuts/blob/main/22.md) - Blind authentication

Non official NUT:
- [x] [NUT-XX](https://github.com/cashubtc/nuts/blob/main/22.md)


## Development

If you want to develop for the project I personally run a hybrid setup. I run the mint locally and the db on docker. 

I have a special development docker compose called: `docker-compose-dev.yml`. This is for simpler development without having traefik in the middle.

The project uses just for building the project and development.

0. Install the dependencies needed to build the node packages and server.

` just install-deps `

1. run the database in docker. 

` docker compose up db -d `

2. Run the mint locally. 
    - You have to write the necessary info into enviroment variables. use the env.example
   file for ideas.

` just run-dev `

### Blinded-message storage

Migration 42 replaces `recovery_signature` with `blind_messages` and `blind_signatures`. Recovery starts at the globally unique blinded point (`B_`) in `blind_messages` and follows its nullable, unique signature reference. `PENDING` messages have no signature; `SIGNED` messages must reference one. Signed messages and signatures are immutable. All signing paths persist both records in the caller's transaction and must return signatures only after commit. Shared sorted transaction-scoped output locks prevent competing writers from claiming the same blinded point.

Known message amounts use positive, uint64-bounded `numeric(20,0)` values. Known mint-quote provenance is retained internally; migrated recovery records do not acquire invented provenance. Signature timestamps remain Unix seconds. Recovery and issuance statistics exclude pending messages. Primary/unique indexes support blinded-point recovery and signature joins; additional indexes support mint-quote references and signing-time statistics. Mint reservation ownership and attempt history are a separate change; migration 42 does not introduce pending-output acquisition or expiry.

Migration runs transactionally and rejects invalid legacy amounts or incomplete DLEQ pairs without discarding the old recovery data. Downgrade refuses pending messages, amounts outside the old `int4` range, known mint provenance, or unlinked signatures rather than losing information. An existing development database that already applied another version of migration 42 requires explicit schema alignment; do not reset it automatically.

Migration 43 moves `melt_change_message` into the same message lifecycle. A non-null `melt_quote_id` identifies change, without a redundant Boolean. Unsigned blanks have a null amount and signature; settlement assigns denominations and links signatures atomically with quote/proof settlement. A unique `(melt_quote_id, output_position)` index preserves submitted order and supports quote lookup and cleanup. Unused blanks (including all blanks for zero refund) are released, definitive failures release unsigned blanks, and uncertain payments retain ownership. Signed change retains its melt provenance and is returned on later quote reads. These are unsigned/signed states, not proof-spending states: unblinded proofs remain unlinkable to these messages.

Legacy change rows have no ordinal, so migration 43 assigns a deterministic order by timestamp and blinded point; their original wallet order cannot be recovered. Duplicate points, overlap with signed records, and missing melt quotes fail the migration transaction without discarding old data. Downgrade refuses any melt-owned messages rather than losing their ordering or provenance. Mint attempts remain deferred to migration 44; no mint-attempt or melt-attempt tracking is introduced here.

Lifecycle database errors are classified by SQLSTATE and stable constraint identifiers, not localized error messages. Application code can use `errors.Is` with the PostgreSQL adapter's `ErrOutputConflict`, `ErrOutputReference`, `ErrOutputInvariant`, `ErrSignedOutputImmutable`, and `ErrUnsafeMigration`; malformed output data uses `cashu.ErrInvalidBlindMessage`. `errors.As` still exposes the original `*pgconn.PgError` for diagnostics. Unique-key failures are not guessed to mean signed versus pending; normal availability checks make that distinction. Internal invariant and migration failures remain generic public errors rather than leaking SQL diagnostics. Migration 43 also updates the reviewed migration 42's trigger functions to provide this metadata on an existing database.

Pending outputs use `cashu.ErrOutputPending` / Cashu code `11004`; pending quotes retain code `20005`. Immediate melt finalization locks and reloads the quote and its complete input set before issuing change. If quote polling already completed settlement, it returns the committed change, fees, and preimage without writing again. Internally paid quotes with pending proofs still complete normal finalization. Missing, replaced, or mixed-state inputs fail without issuing change. Migration 43 includes a partial `proofs(quote)` index for these authoritative input lookups.

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
