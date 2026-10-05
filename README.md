# StoreLink Backend

## Gerar o executável

No PowerShell, execute:

```powershell
cd D:\WorkspacePhSoftwares\StoreLink\storelink-backend
go build -o .\storelink-backend.exe .\cmd\api
```

O executável será gerado em:

```text
D:\WorkspacePhSoftwares\StoreLink\storelink-backend\storelink-backend.exe
```

## Banco local

Para o ambiente local de teste:

```text
Usuário: postgres
Senha: postgres
Banco: storelink
```

String de conexão:

```text
postgres://postgres:postgres@127.0.0.1:5432/storelink?sslmode=disable
```

Ao iniciar, a API carrega automaticamente um arquivo `.env` localizado na
pasta do executável ou no diretório atual. Variáveis já definidas pelo serviço
ou pelo processo têm prioridade sobre o arquivo.
