# ssip-demo-project

Demo para a [SSIP-Trusted-Pipeline](https://github.com/Laerson/ssip-trusted-pipeline)

## Visão Geral

Este projeto é um exemplo de como utilizar o workflow reusável da SSIP-Trusted-Pipeline. O [Workflow deste projeto](.github/workflows/untrusted-pipeline.yml) invoca o workflow reusável da SSIP-Trusted-Pipeline, que é responsável por realizar testes de conformidade e construir uma imagem Docker, junto com evidências seguras do processo de construção.

## Verificação dos atestados

O workflow gera e anexa os seguintes atestados assinados:

1. **SLSA Provenance**
2. **SBOM**
3. **Scan de Vulnerabilidade**
4. **Resultado do SAST**
5. **Resultado dos testes de unidade**

### 1. Pré requisitos

Para verificar os atestados é necessário ter o cosign instalado na máquina.

Se você tiver Go 1.20+ instalado, você pode instalar o cosign com o seguinte comando:

```bash
go install github.com/sigstore/cosign/v2/cmd/cosign@latest
```

Outras opções de instalação podem ser encontradas na [documentação oficial](https://docs.sigstore.dev/system_config/installation).

Caso a imagem esteja em um repositório privado, é necessário fazer autenticação usando o comando `cosign login`.

```bash
  # Log in em reg.example.com, com usuário AzureDiamond e senha hunter2
  cosign login reg.example.com -u AzureDiamond -p hunter2
```

### 2. Verificação dos atestados

Para verificar os atestados, execute o seguinte comando:

```bash
cosign verify-attestation --type=<tipo de predicado> \
--certificate-identity <ref da workflow reusável> \
--certificate-oidc-issuer https://token.actions.githubusercontent.com \
--certificate-github-workflow-ref <ref do workflow que invocou o workflow reusável>
<nome da imagem>
```

Para possíveis valores de `<tipo de predicado>`, e outras opções disponíveis, consulte a [documentação do comando cosign verify-attestation](https://github.com/sigstore/cosign/blob/main/doc/cosign_verify-attestation.md#options).

Se a flag `--type` for omitida, o cosign irá verificar todos os atestados anexados na imagem.

Exemplo:

```bash
cosign verify-attestation --type=slsaprovenance \
  --certificate-identity "https://github.com/laerson/ssip-trusted-pipeline/.github/workflows/trusted-pipeline.yml@refs/tags/v1.0.0" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-github-workflow-ref refs/tags/v1.0.0 \
  --certificate-github-workflow-name "Untrusted Pipeline" \
  --certificate-github-workflow-repository Laerson/ssip-demo-project \
  --certificate-github-workflow-trigger workflow_dispatch \
  ghcr.io/laerson/ssip-demo-project:v1.0.0
```

## Download dos atestados

É possível baixar os atestados anexados na imagem Docker utilizando o comando `cosign download attestation`.

[Documentação do comando cosign download attestation](https://github.com/sigstore/cosign/blob/main/doc/cosign_download_attestation.md)

A saída do comando é um bundle `.jsonl` codificado em base64, contendo todos os atestados anexados na imagem.

exemplo:

```bash
cosign download attestation ghcr.io/laerson/ssip-demo-project:v1.0.0 > encoded-artifact.intoto.jsonl
```

Para decodificar o bundle, utilize o seguinte comando:

```bash
cat encoded-artifact.intoto.jsonl | jq -r '.payload' | base64 -d | jq -s >> artifact.intoto.json
```

É possível filtrar os atestados por tipo de predicado usando a flag `--predicate-type`.
