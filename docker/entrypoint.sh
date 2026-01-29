#!/bin/bash
# Kubrowser Terminal Entrypoint
#
# Creates a user matching KUBROWSER_USER env var,
# sets up their home directory, and switches to that user.

set -e

# Get username from environment (default: kubrowser)
USERNAME="${KUBROWSER_USER:-kubrowser}"
HOME_DIR="/home/${USERNAME}"

# Create user if they don't exist
if ! id "$USERNAME" &>/dev/null; then
    # Create user with UID 1000
    adduser -D -u 1000 -s /bin/bash -h "$HOME_DIR" "$USERNAME" 2>/dev/null || true
fi

# Ensure home directory exists and is owned by user
mkdir -p "$HOME_DIR"
chown -R 1000:1000 "$HOME_DIR"

# Create .bashrc if it doesn't exist
if [ ! -f "$HOME_DIR/.bashrc" ]; then
    cat > "$HOME_DIR/.bashrc" << 'EOF'
# Kubrowser Terminal Configuration

# Custom prompt: username@kubrowser:path$
export PS1='\[\033[01;32m\]\u@kubrowser\[\033[00m\]:\[\033[01;34m\]\w\[\033[00m\]\$ '

# Aliases
alias ll='ls -la'
alias k='kubectl'
alias kgp='kubectl get pods'
alias kgs='kubectl get svc'
alias kgd='kubectl get deployments'
alias kgn='kubectl get nodes'

# History settings
export HISTSIZE=10000
export HISTFILESIZE=20000
export HISTCONTROL=ignoreboth:erasedups
shopt -s histappend

# Enable color support
alias ls='ls --color=auto'
alias grep='grep --color=auto'

# Welcome message
echo "Welcome to Kubrowser Terminal!"
echo "Type 'k' as shorthand for 'kubectl'"
echo ""
EOF
    chown 1000:1000 "$HOME_DIR/.bashrc"
fi

# Create .bash_profile to source .bashrc
if [ ! -f "$HOME_DIR/.bash_profile" ]; then
    echo 'source ~/.bashrc' > "$HOME_DIR/.bash_profile"
    chown 1000:1000 "$HOME_DIR/.bash_profile"
fi

# Set environment for the user
export USER="$USERNAME"
export HOME="$HOME_DIR"
export SHELL=/bin/bash

# Switch to the user and run the command
cd "$HOME_DIR"
exec su-exec "$USERNAME" "$@"
