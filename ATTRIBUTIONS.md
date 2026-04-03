# Attribution

Merton is here because of the hero devs who did the real work.
These are the projects that made it possible.

---

## go-psrp / go-psrpcore

**Jason Simons** (smnsjas)
https://github.com/smnsjas/go-psrp

A Go implementation of the PowerShell Remoting Protocol. It's doing a lot of the core work inside Merton. I modified it for a CTF/pentesting context, but much of the core remains. Merton will probably keep growing away from his big brother, but he's learned a lot along the way.

This is itself a Go port of **pypsrp** and **psrp-core** by **Jordan Borean**, whose Python implementation is the OG. 
https://github.com/jborean93/pypsrp

---

## ntlmssp
**bodgit**
https://github.com/bodgit/ntlmssp

Clean NTLM implementation in Go. The project went quiet but the foundation was good enough that it was worth picking back up. A 20-byte size miscalculation in the NTLM message structure caused authentication to fail silently. I patched that, added pass-the-hash support, and Merton moves forward slow and steady.

---

## go-krb5
https://github.com/go-krb5/krb5

Kerberos library for Go. I forked this, merged context propagation work from smnsjas, and made a few other additions. It'll get a little more lovings down the road, some newer encryption mechanisms aren't built yet but it got Merton to where it needed to be for now.

---

## prompt
**nao1215**
https://github.com/nao1215/prompt

Terminal prompt handling. I touched this minimally and mainly to fix an issue I had with paste detection.

---

If I used your work and got something wrong here, I am sorry, please let me know!
