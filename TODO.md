# TODO

## spacekeeper

- Run the opt-in periodic-save (`make save-agent-install`) and login-restore (`make restore-agent-install`) agents for a while; tune the 180s interval / 25s settle delay and the `-keep` count once they prove out.
- Multi-display space creation assumes SLS display order matches the Mission Control AX `mc.display` order. Held on this 2-display setup; revisit if it ever creates desktops on the wrong display.
- AX-title fallback only covers the active space (AX enumerates ~current-space windows); full titles still need Screen Recording. Fine, just noting the ceiling.
