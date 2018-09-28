# Hydra Google Cloud Datastore Plugin

This introduces a `datastore` URL option for hydra that leverages Google's Cloud Datastore

```go
// Datastore URLs should be in the format of datastore://<projectid>?namespace=&credentialsFile=
// Just using datastore:// will be sufficient if running on GCP wiith an DATASTORE_PROJECT_ID env var set
```

Compile as follows:

```shell
$ go build -buildmode=plugin -o=datastore.so github.com/someone1/hydra-gcp/plugin
```

Then launch hydra with the following options:

```shell
    DATABASE_URL=datastore://<projectid>?namespace=&credentialsFile=
    DATABASE_PLUGIN=datastore.so
```

**NOTE:** This does not support rotating the encryption key used for storing data in hydra (yet?)
