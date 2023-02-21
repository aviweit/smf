# Tests

Environment: free5gc-compose of three UPFs invoked with smf built from PR [#69](https://github.com/free5gc/smf/pull/69).

## Test 1 (delete UPF after UPF-2 is added)

1. Start 5gcore with [smfcfg.yaml](https://github.com/free5gc/free5gc-compose/blob/master/config/smfcfg.yaml)

2. Ensure `upNodes` and `links` properly reported: `curl -X GET http://127.0.0.1:8000/upi/v1/upNodesLinks`

   Expected: gNB, UPF returned as `upNodes`, gNB -> UPF as `links`

3. Log into ueransim (UE): `docker compose exec ueransim bash`

4. Create session: `./nr-ue -c config/uecfg.yaml`

   Expected: session created into `UPF`; tcpdump in `UPF` shows UE traffic

5. Add [UPF-2](./upNodes-upf-2.json): `curl -X POST -d "@upNodes-upf-2.json" http://127.0.0.1:8000/upi/v1/upNodesLinks`

6. Ensure `upNodes` and `links` properly updated: `curl -X GET http://127.0.0.1:8000/upi/v1/upNodesLinks`

   Expected: gNB, UPF, UPF-2 returned as `upNodes`, gNB -> UPF, gNB -> UPF-2 as `links`

7. Delete UPF: `curl -X DELETE http://127.0.0.1:8000/upi/v1/upNodesLinks/UPF`

   Expected: session get re-created into `UPF-2`; tcpdump in `UPF-2` shows UE traffic

8. Ensure `upNodes` and `links` properly updated: `curl -X GET http://127.0.0.1:8000/upi/v1/upNodesLinks`

   Expected: gNB, UPF-2 returned as `upNodes`, gNB -> UPF-2 as `links`


## Test 2 (delete UPF after UPF-2 is added, then add UPF again)

1. Start 5gcore with [smfcfg.yaml](https://github.com/free5gc/free5gc-compose/blob/master/config/smfcfg.yaml)

2. Ensure `upNodes` and `links` properly reported: `curl -X GET http://127.0.0.1:8000/upi/v1/upNodesLinks`

   Expected: gNB, UPF returned as `upNodes`, gNB -> UPF as `links`

3. Log into ueransim (UE): `docker compose exec ueransim bash`

4. Create session: `./nr-ue -c config/uecfg.yaml`

   Expected: session created into `UPF`; tcpdump in `UPF` shows UE traffic

5. Add [UPF-2](./upNodes-upf-2.json): `curl -X POST -d "@upNodes-upf-2.json" http://127.0.0.1:8000/upi/v1/upNodesLinks`

6. Ensure `upNodes` and `links` properly updated: `curl -X GET http://127.0.0.1:8000/upi/v1/upNodesLinks`

   Expected: gNB, UPF, UPF-2 returned as `upNodes`, gNB -> UPF, gNB -> UPF-2 as `links`

7. Delete UPF: `curl -X DELETE http://127.0.0.1:8000/upi/v1/upNodesLinks/UPF`

   Expected: session get re-created into `UPF-2`; tcpdump in `UPF-2` shows UE traffic

8. Ensure `upNodes` and `links` properly updated: `curl -X GET http://127.0.0.1:8000/upi/v1/upNodesLinks`

   Expected: gNB, UPF-2 returned as `upNodes`, gNB -> UPF-2 as `links`

9. Add [UPF](./upNodes-upf-1.json): `curl -X POST -d "@upNodes-upf-1.json" http://127.0.0.1:8000/upi/v1/upNodesLinks`

10. Ensure `upNodes` and `links` properly updated: `curl -X GET http://127.0.0.1:8000/upi/v1/upNodesLinks`

   Expected: gNB, UPF, UPF-2 returned as `upNodes`, gNB -> UPF, gNB -> UPF-2 as `links`

11. Delete UPF-2: `curl -X DELETE http://127.0.0.1:8000/upi/v1/upNodesLinks/UPF-2`

   Expected: session get re-created into `UPF`; tcpdump in `UPF` shows UE traffic


## Test 3 (delete UPF before UPF-2 is added)

1. Start 5gcore with [smfcfg.yaml](https://github.com/free5gc/free5gc-compose/blob/master/config/smfcfg.yaml)

2. Ensure `upNodes` and `links` properly reported: `curl -X GET http://127.0.0.1:8000/upi/v1/upNodesLinks`

   Expected: gNB, UPF returned as `upNodes`, gNB -> UPF as `links`

3. Log into ueransim (UE): `docker compose exec ueransim bash`

4. Create session: `./nr-ue -c config/uecfg.yaml`

   Expected: session created into UPF; tcpdump in UPF shows UE traffic

5. Delete UPF: `curl -X DELETE http://127.0.0.1:8000/upi/v1/upNodesLinks/UPF`

   Expected: session get re-created with failure of `INSUFFICIENT_RESOURCES_FOR_SPECIFIC_SLICE_AND_DNN`

6. Ensure `upNodes` and `links` properly reported: `curl -X GET http://127.0.0.1:8000/upi/v1/upNodesLinks`

   Expected: gNB, returned as `upNodes`, [] as `links`

7. Add [UPF-2](./upNodes-upf-2.json): `curl -X POST -d "@upNodes-upf-2.json" http://127.0.0.1:8000/upi/v1/upNodesLinks`

   Expected: session get re-created into `UPF-2`; tcpdump in `UPF-2` shows UE traffic

8. Ensure `upNodes` and `links` properly reported: `curl -X GET http://127.0.0.1:8000/upi/v1/upNodesLinks`

   Expected: gNB, UPF-2 returned as `upNodes`, gNB -> UPF-2 as `links`


## Test 4 (delete selected anchor UPF)

1. Start 5gcore with [smfcfg.yaml](https://github.com/free5gc/free5gc-compose/blob/master/config/smfcfg.yaml)

2. Ensure `upNodes` and `links` properly reported: `curl -X GET http://127.0.0.1:8000/upi/v1/upNodesLinks`

   Expected: gNB, UPF returned as `upNodes`, gNB -> UPF as `links`

3. Add [UPF-2](./upNodes-upf-1-2.json): `curl -X POST -d "@upNodes-upf-1-2.json" http://127.0.0.1:8000/upi/v1/upNodesLinks`

4. Add [UPF-3](./upNodes-upf-1-3.json): `curl -X POST -d "@upNodes-upf-1-3.json" http://127.0.0.1:8000/upi/v1/upNodesLinks`

5. Ensure `upNodes` and `links` properly reported: `curl -X GET http://127.0.0.1:8000/upi/v1/upNodesLinks`

   Expected: gNB, UPF, UPF-2, UPF-3 returned as `upNodes`, gNB -> UPF, UPF -> UPF-2, UPF -> UPF-3 as `links`

3. Log into ueransim (UE): `docker compose exec ueransim bash`

4. Create session: `./nr-ue -c config/uecfg.yaml`

   Expected: session created into either UPF-2 or UPF-3; tcpdump in `UPF` and selected anchor upf shows UE traffic

5. Delete UPF: `curl -X DELETE http://127.0.0.1:8000/upi/v1/upNodesLinks/<selected anchor upf>`

   Expected: session get re-created into the other anchor upf

6. Ensure `upNodes` and `links` properly reported: `curl -X GET http://127.0.0.1:8000/upi/v1/upNodesLinks`

   Expected: gNB, UPF, other_anchor_upf, returned as `upNodes`, gNB -> UPF, UPF -> other_anchor_upf as `links`

## Test 4, 5, 6, 7

Repeat the above tests (Test 1, Test 2, Test 3, Test 4) using this [smfcfg-heartbeat.yaml](./smfcfg-heartbeat.yaml).

Expected: after every "Delete UPF" step - heartbeat thread of this UPF is canceled
