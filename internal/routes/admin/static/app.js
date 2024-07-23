



/**
    * @typedef {Object}  UnsignedNostrEvent
    * @property {number} created_at  - should be a unix timestamp
    * @property {number} kind
    * @property {Array[][]} tags
    * @property {string} content
*/
/**
    * @typedef {Object}  SignedNostrEvent
    * @property {number} created_at  - should be a unix timestamp
    * @property {number} kind
    * @property {Array[][]} tags
    * @property {string} content
    * @property {string} id
    * @property {string} sig
*/



let loginForm = document.getElementById("login-form")
loginForm?.addEventListener("submit", (e) => {
    e.preventDefault();

    let formValues = Object.values(e.target).reduce((obj,field) => { obj[field.name] = field.value; return obj }, {})

    console.log({formValues})


    /** @type {UnsignedNostrEvent}*/
    const eventToSign = {
        created_at: Math.floor(Date.now() / 1000),
        kind: 27235, 
        tags: [],
        content: formValues.passwordNonce
    }

    console.log({target: e.target})
    window.nostr.signEvent(eventToSign).then((/** 
        @type {SignedNostrEvent}
        */signedEvent) => {
        console.log({signedEvent})

        const loginRequest = new Request("/admin/login", {method: "POST", body: JSON.stringify(signedEvent)})

            fetch(loginRequest).then((_) => {
                window.location.href="/admin"
            }).catch(err => {
                console.log({err})
            })
        // request mint login

        fetch()
    }).catch((err) => {
        console.log({err})
    })

    // console.log({nostr: window.nostr.get-nostr-key})


})
