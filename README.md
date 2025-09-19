# API de implantação de OpenTelemetry

## Execução

* Docker
```cmd
    docker compose up --build
```

* Taskfile
```cmd
    task up
```

* Testar
Arquivo api.http contém os testes para o teste da API.


## API

Retornos

200 - {"temp_C":28.5,"temp_F":83.3,"temp_K":301.5}
422 - {"error":"invalid zipcode"} (cep inválido)
404 - {"error":"can not find zipcode"} (cep não encontrado)
