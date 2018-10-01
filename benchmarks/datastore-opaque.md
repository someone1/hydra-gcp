# Hydra GCP Performance Benchmarks _(token strategy=opaque, database=datastore)_

In this document you will find benchmark results for different endpoints of ORY Hydra. All benchmarks are executed
using [rakyll/hey](https://github.com/rakyll/hey). Please note that these benchmarks run against the datastore storage
adapter.

Please note performance may greatly differs between deployments (e.g. request latency, database configuration) and
tweaking individual things may greatly improve performance. This is also not indicative of long-term performance as
database sizes grow. Take these results with a very large grain of salt.

All benchmarks run 10,000 requests in total, with 100 concurrent requests. All benchmarks run on a n1-highcpu-4
Compute Engine VM (4 vCPUs, 3.6GB memory) in the us-east1-b zone on Google Compute Engine. To provide enough entropy
for random number generation, haveged was installed and setup on the test machine. Where applicable, the
n1-standard-2 (2 vCPUs, 7.5 GB memory) machine type is used for the PostgreSQL instance using Google Cloud SQL
running in the same zone.

## BCrypt

ORY Hydra uses BCrypt to obfuscate secrets of OAuth 2.0 Clients. When using flows such as the OAuth 2.0 Client Credentials
Grant, ORY Hydra validates the client credentials using BCrypt which causes (by design) CPU load. CPU load and performance
depend on the BCrypt cost which can be set using the environment variable `BCRYPT_COST`. For these benchmarks,
we have set `BCRYPT_COST=8`.

## OAuth 2.0

This section contains various benchmarks against OAuth 2.0 endpoints

### Token Introspection

```

Summary:
  Total:	10.9622 secs
  Slowest:	0.3740 secs
  Fastest:	0.0737 secs
  Average:	0.1039 secs
  Requests/sec:	912.2266
  
  Total data:	1550000 bytes
  Size/request:	155 bytes

Response time histogram:
  0.074 [1]	|
  0.104 [6843]	|■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■
  0.134 [2258]	|■■■■■■■■■■■■■
  0.164 [256]	|■
  0.194 [308]	|■■
  0.224 [154]	|■
  0.254 [70]	|
  0.284 [74]	|
  0.314 [22]	|
  0.344 [8]	|
  0.374 [6]	|


Latency distribution:
  10% in 0.0813 secs
  25% in 0.0857 secs
  50% in 0.0947 secs
  75% in 0.1075 secs
  90% in 0.1280 secs
  95% in 0.1800 secs
  99% in 0.2601 secs

Details (average, fastest, slowest):
  DNS+dialup:	0.0000 secs, 0.0737 secs, 0.3740 secs
  DNS-lookup:	0.0000 secs, 0.0000 secs, 0.0061 secs
  req write:	0.0000 secs, 0.0000 secs, 0.0014 secs
  resp wait:	0.1038 secs, 0.0736 secs, 0.3656 secs
  resp read:	0.0001 secs, 0.0000 secs, 0.0029 secs

Status code distribution:
  [200]	10000 responses



```

### Client Credentials Grant

This endpoint uses [BCrypt](#bcrypt).

```

Summary:
  Total:	62.6571 secs
  Slowest:	1.4795 secs
  Fastest:	0.1267 secs
  Average:	0.6108 secs
  Requests/sec:	159.5989
  
  Total data:	1570000 bytes
  Size/request:	157 bytes

Response time histogram:
  0.127 [1]	|
  0.262 [130]	|■■
  0.397 [855]	|■■■■■■■■■■■
  0.533 [2627]	|■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■
  0.668 [3121]	|■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■
  0.803 [1771]	|■■■■■■■■■■■■■■■■■■■■■■■
  0.938 [982]	|■■■■■■■■■■■■■
  1.074 [329]	|■■■■
  1.209 [127]	|■■
  1.344 [43]	|■
  1.480 [14]	|


Latency distribution:
  10% in 0.3985 secs
  25% in 0.4818 secs
  50% in 0.5881 secs
  75% in 0.7146 secs
  90% in 0.8657 secs
  95% in 0.9413 secs
  99% in 1.1558 secs

Details (average, fastest, slowest):
  DNS+dialup:	0.0000 secs, 0.1267 secs, 1.4795 secs
  DNS-lookup:	0.0000 secs, 0.0000 secs, 0.0060 secs
  req write:	0.0000 secs, 0.0000 secs, 0.0109 secs
  resp wait:	0.6106 secs, 0.1265 secs, 1.4795 secs
  resp read:	0.0001 secs, 0.0000 secs, 0.0102 secs

Status code distribution:
  [200]	10000 responses



```

## OAuth 2.0 Client Management

### Creating OAuth 2.0 Clients

This endpoint uses [BCrypt](#bcrypt) and generates IDs and secrets by reading from  which negatively impacts
performance. Performance will be better if IDs and secrets are set in the request as opposed to generated by Hydra GCP.

```
This test is currently disabled due to issues with /dev/urandom being inaccessible in the CI.
```

### Listing OAuth 2.0 Clients

```

Summary:
  Total:	5.2884 secs
  Slowest:	0.2344 secs
  Fastest:	0.0363 secs
  Average:	0.0496 secs
  Requests/sec:	1890.9376
  
  Total data:	4150000 bytes
  Size/request:	415 bytes

Response time histogram:
  0.036 [1]	|
  0.056 [8672]	|■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■
  0.076 [918]	|■■■■
  0.096 [179]	|■
  0.116 [70]	|
  0.135 [77]	|
  0.155 [39]	|
  0.175 [25]	|
  0.195 [13]	|
  0.215 [4]	|
  0.234 [2]	|


Latency distribution:
  10% in 0.0404 secs
  25% in 0.0424 secs
  50% in 0.0457 secs
  75% in 0.0510 secs
  90% in 0.0585 secs
  95% in 0.0699 secs
  99% in 0.1307 secs

Details (average, fastest, slowest):
  DNS+dialup:	0.0000 secs, 0.0363 secs, 0.2344 secs
  DNS-lookup:	0.0000 secs, 0.0000 secs, 0.0044 secs
  req write:	0.0000 secs, 0.0000 secs, 0.0021 secs
  resp wait:	0.0495 secs, 0.0362 secs, 0.2344 secs
  resp read:	0.0000 secs, 0.0000 secs, 0.0033 secs

Status code distribution:
  [200]	10000 responses



```

### Fetching a specific OAuth 2.0 Client

```

Summary:
  Total:	5.1956 secs
  Slowest:	0.2007 secs
  Fastest:	0.0355 secs
  Average:	0.0478 secs
  Requests/sec:	1924.7092
  
  Total data:	4130000 bytes
  Size/request:	413 bytes

Response time histogram:
  0.036 [1]	|
  0.052 [8512]	|■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■
  0.069 [1113]	|■■■■■
  0.085 [103]	|
  0.102 [53]	|
  0.118 [69]	|
  0.135 [60]	|
  0.151 [49]	|
  0.168 [24]	|
  0.184 [9]	|
  0.201 [7]	|


Latency distribution:
  10% in 0.0394 secs
  25% in 0.0410 secs
  50% in 0.0444 secs
  75% in 0.0489 secs
  90% in 0.0552 secs
  95% in 0.0633 secs
  99% in 0.1307 secs

Details (average, fastest, slowest):
  DNS+dialup:	0.0000 secs, 0.0355 secs, 0.2007 secs
  DNS-lookup:	0.0000 secs, 0.0000 secs, 0.0081 secs
  req write:	0.0000 secs, 0.0000 secs, 0.0041 secs
  resp wait:	0.0476 secs, 0.0354 secs, 0.2006 secs
  resp read:	0.0000 secs, 0.0000 secs, 0.0046 secs

Status code distribution:
  [200]	10000 responses



```