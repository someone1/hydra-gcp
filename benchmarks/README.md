# Hydra GCP Benchmarks

## Intro

The benchmarks found here are meant to be relatively compared to one another - they do not in anyway depict real-world
performance of a Hydra GCP deployment. There are many factors that may affect performance such as hardware and environment
used.

These tests show how hydra runs in various configurations on the SAME hardware - so relative performance can be observed.
The goal here was to see how using Cloud Datastore and/or the IAM API would affect overall throughput of hydra. The
tests were done on Google Cloud Platform, utilizing a n1-highcpu-4 Compute Engine VM (4 vCPUs, 3.6 GB memory) to run the
benchmarks. Google Cloud SQL with PostgreSQL 9.6 was used for the PostgreSQL tests, which was running in a n1-standard-2
VM (2 vCPUs, 7.5 GB memory). All hosts were running in the us-east1-b region. A fresh database and clean Datastore was
used for each benchmark where applicable.

## Results

The results mostly speak for themselves. The goal of this exercise was to see the overall impact of utilizing Google's
Cloud infrastructure for the database and/or signing of tokens for Hydra. Overall, the variance in latency is negligible,
especially considering that Google's Cloud Datastore will scale and perform consistently with 0 devops work. The IAM API
definitely has an overhead but has the benefit of a fully managed, automatic key rotating system.

**95th Percentile Results**
| (Database, Token Type) | Introspection | Client Credentials Grant | Listing Clients | Fetching Client |
| --- | --- | --- | --- | --- |
| [In-memory, opaque](https://github.com/someone1/hydra-gcp/tree/master/benchmarks/memory-opaque.md) | 0.0181 secs | 0.8478 secs | 0.0218 secs | 0.0209 secs |
| [In-memory, jwt](https://github.com/someone1/hydra-gcp/tree/master/benchmarks/memory-jwt.md) | 0.0464 secs | 3.8286 secs | 0.0206 secs | 0.0187 secs |
| [In-memory, IAM API](https://github.com/someone1/hydra-gcp/tree/master/benchmarks/memory-iam.md) | 0.0473 secs | 1.0458 secs | 0.0110 secs | 0.0097 secs |
| [PostgreSQL, opaque](https://github.com/someone1/hydra-gcp/tree/master/benchmarks/postgres-opaque.md) | 0.0930 secs | 2.1605 secs | 0.0561 secs | 0.0469 secs |
| [Datastore, opaque](https://github.com/someone1/hydra-gcp/tree/master/benchmarks/datastore-opaque.md) | 0.1800 secs | 0.9413 secs | 0.0699 secs | 0.0633 secs |
| [Datastore, IAM API](https://github.com/someone1/hydra-gcp/tree/master/benchmarks/datastore-iam.md) | 0.1522 secs | 1.0403 secs | 0.0644 secs | 0.0620 secs |
