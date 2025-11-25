// Authentication module for NIP-07 login form
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

/**
 * Initialize NIP-07 login form handling
 */
export function initAuth() {
  let nip07form = document.getElementById("nip07-form");
  // sig nonce sent by the server, in case of success navigate. if an error occurs show an error
  nip07form?.addEventListener("submit", (e) => {
    e.preventDefault();

    let formValues = Object.values(e.target).reduce((obj, field) => {
      obj[field.name] = field.value;
      return obj;
    }, {});

    /** @type {UnsignedNostrEvent}*/
    const eventToSign = {
      created_at: Math.floor(Date.now() / 1000),
      kind: 27235,
      tags: [],
      content: formValues.passwordNonce,
    };

    window.nostr
      .signEvent(eventToSign)
      .then(
        (
          /**
          @type {SignedNostrEvent}
          */ signedEvent
        ) => {
          const loginRequest = new Request("/admin/login", {
            method: "POST",
            body: JSON.stringify(signedEvent),
          });

          fetch(loginRequest)
            .then(async (res) => {

      const text = await res.text();
      if (res.ok) {
        const targetHeader = res.headers.get("HX-RETARGET");
        if (targetHeader) {
          window.htmx.swap(`${targetHeader}`, text, { swapStyle: "innerHTML" });
          return
        }

          window.location.href = "/admin";
      } else {
        const targetHeader = res.headers.get("HX-RETARGET");
        if (window.htmx && targetHeader) {
          window.htmx.swap(`#${targetHeader}`, text, { swapStyle: "innerHTML" });
        }
      }
            })
            .catch((err) => {
              console.log("Error message");
              console.log({ err });
            });
        }
      )
      .catch((err) => {
        console.log({ err });
      });
  });
}
