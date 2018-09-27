# Hydra Google Cloud Datastore Plugin

Compile as follows:

```shell
$ go build -buildmode=plugin -o=datastore.so github.com/someone1/hydra-gcp/plugin
```

**NOTE:** This does not support rotating the encryption key used for storing data in hydra (yet?)
