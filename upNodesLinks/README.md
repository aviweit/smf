# Tests

free5gc-compose of three UPFs invoked with smf built from PR #69.

## 1

### Flow

1. Start 5gcore with [smfcfg.yaml](https://github.com/free5gc/free5gc-compose/blob/master/config/smfcfg.yaml)

2. Ensure `upNodes` and `links` properly reported: `curl -X GET http://127.0.0.1:8000/upi/v1/upNodesLinks`

   Expected: AN, UPF returned as `upNodes`, gNB -> UPF as `links`

3. Log into ueransim (UE): `docker compose exec ueransim bash`

4. Create session: `sudo ./nr-ue -c config/uecfg.yaml`

   Expected: session created into `UPF`; tcpdump in `UPF` shows UE traffic

5. Add [UPF-2](./upNodes-upf-2.json): `curl -X POST -d "@upNodes-upf-2.json" http://127.0.0.1:8000/upi/v1/upNodesLinks`

6. Ensure `upNodes` and `links` properly updated: `curl -X GET http://127.0.0.1:8000/upi/v1/upNodesLinks`

   Expected: AN, UPF, UPF-2 returned as `upNodes`, gNB -> UPF, gNB -> UPF-2 as `links`

7. Delete UPF: `curl -X DELETE http://127.0.0.1:8000/upi/v1/upNodesLinks/UPF`

   Expected: session get re-created into `UPF-2`; tcpdump in `UPF-2` shows UE traffic

8. Ensure `upNodes` and `links` properly updated: `curl -X GET http://127.0.0.1:8000/upi/v1/upNodesLinks`

   Expected: AN, UPF-2 returned as `upNodes`, gNB -> UPF-2 as `links`


## 2

1. Start 5gcore with [smfcfg.yaml](https://github.com/free5gc/free5gc-compose/blob/master/config/smfcfg.yaml)

2. Ensure `upNodes` and `links` properly reported: `curl -X GET http://127.0.0.1:8000/upi/v1/upNodesLinks`

   Expected: AN, UPF returned as `upNodes`, gNB -> UPF as `links`

3. Log into ueransim (UE): `docker compose exec ueransim bash`

4. Create session: `sudo ./nr-ue -c config/uecfg.yaml`

   Expected: session created into UPF; tcpdump in UPF shows UE traffic

5. Delete UPF: `curl -X DELETE http://127.0.0.1:8000/upi/v1/upNodesLinks/UPF`

   Expected: session get re-created with failure of `INSUFFICIENT_RESOURCES_FOR_SPECIFIC_SLICE_AND_DNN`

6. Ensure `upNodes` and `links` properly reported: `curl -X GET http://127.0.0.1:8000/upi/v1/upNodesLinks`

   Expected: AN, returned as `upNodes`, [] as `links`

7. Add [UPF-2](./upNodes-upf-2.json): `curl -X POST -d "@upNodes-upf-2.json" http://127.0.0.1:8000/upi/v1/upNodesLinks`

   Expected: session get re-created into `UPF-2`; tcpdump in `UPF-2` shows UE traffic

8. Ensure `upNodes` and `links` properly reported: `curl -X GET http://127.0.0.1:8000/upi/v1/upNodesLinks`

   Expected: AN, UPF-2 returned as `upNodes`, gNB -> UPF-2 as `links`

