![powered by vibes](https://img.shields.io/badge/powered%20by-vibes-7c3aed?style=flat)

# Kubrowser
A browser-based terminal for my Kubernetes homelab. Lets me give friends shell access to my cluster through temporary pods without handing out SSH keys or teaching them how to configure kubectl. I'm fully expecting them to break my cluster, but let's be honest, building it was half the fun.


> Kubrowser was 100% vibe coded while holding a baby in one arm. I might polish it up later. Maybe.



## Features

- Browser-based terminal with WebSocket TTY
- Automatic pod creation and cleanup
- Isolated sessions per user
- Pod and node commads trigger popouts which show results and add shortcut buttons
