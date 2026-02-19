#!/bin/bash
# =============================================================================
# setup.sh — Inicializa as dependências do plugin segi9
#
# Execute UMA VEZ antes do primeiro build:
#   chmod +x setup.sh
#   ./setup.sh
# =============================================================================

set -e

echo "==> Verificando Go..."
if ! command -v go &>/dev/null; then
    echo "ERRO: Go não encontrado. Instale em https://go.dev/dl/"
    exit 1
fi
go version

echo ""
echo "==> Buscando o último commit da branch release/7.4 do SDK..."
echo "    (se aparecer erro 'Forbidden', veja as instruções abaixo)"
echo ""

# Tenta usar o go get diretamente (funciona quando o servidor de módulos
# do Zabbix está acessível com o GOPROXY padrão).
go get golang.zabbix.com/sdk@af85407 || {
    echo ""
    echo "------------------------------------------------------------------------"
    echo "AVISO: Não foi possível baixar o SDK automaticamente."
    echo ""
    echo "Opções:"
    echo ""
    echo "1) Tente especificar um hash mais recente manualmente:"
    echo "   Acesse: https://git.zabbix.com/projects/AP/repos/plugin-support/commits"
    echo "           (branch: release/7.4)"
    echo "   Copie o hash do último commit e execute:"
    echo "   go get golang.zabbix.com/sdk@<HASH>"
    echo ""
    echo "2) Clone o SDK localmente e use um 'replace' no go.mod:"
    echo "   git clone https://git.zabbix.com/scm/ap/plugin-support.git ../sdk"
    echo "   go mod edit -replace=golang.zabbix.com/sdk=../sdk"
    echo "   go mod tidy"
    echo "------------------------------------------------------------------------"
    exit 1
}

echo ""
echo "==> Executando go mod tidy..."
go mod tidy

echo ""
echo "==> Pronto! Agora execute: make build"
echo ""
