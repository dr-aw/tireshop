FROM mcr.microsoft.com/devcontainers/base:ubuntu24.04 AS development

ARG MISE_VERSION=2026.9.11

USER root

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        build-essential \
        pkg-config \
        jq \
        ripgrep \
        fd-find \
        unzip \
        postgresql-client \
        libglib2.0-0t64 libnspr4 libnss3 \
        libatk1.0-0t64 libatk-bridge2.0-0t64 libatspi2.0-0t64 \
        libdbus-1-3 libx11-6 libxcomposite1 libxdamage1 libxext6 \
        libxfixes3 libxrandr2 libgbm1 libxcb1 libxkbcommon0 libasound2t64 \
        fonts-liberation fontconfig \
    && rm -rf /var/lib/apt/lists/*

# Install the official mise binary.
RUN curl -fsSL https://mise.run \
    | MISE_VERSION="${MISE_VERSION}" \
      MISE_INSTALL_PATH="/usr/local/bin/mise" \
      sh

RUN mise --version

# Enable mise automatically in interactive shells.
RUN echo 'eval "$(mise activate bash)"' >> /home/vscode/.bashrc \
    && echo 'eval "$(mise activate zsh)"' >> /home/vscode/.zshrc

USER vscode

WORKDIR /workspace

# Named volumes inherit these permissions on their first mount.
RUN mkdir -p /home/vscode/.local/share/mise /home/vscode/.codex \
    && chmod 700 /home/vscode/.codex

ENV PATH="/home/vscode/.local/share/mise/shims:${PATH}"

USER root
COPY .devcontainer/bootstrap.sh /usr/local/bin/tireshop-bootstrap
RUN chmod 755 /usr/local/bin/tireshop-bootstrap
USER vscode

FROM development AS agent
USER root
RUN rm -f /etc/sudoers.d/vscode
USER vscode
