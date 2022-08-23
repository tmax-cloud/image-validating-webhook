# Quick Start Guide

1. for administrator of cluster :
    - If you want to except some images or namespaces from validation, add it to white list config map named `image-validation-webhook-whitelist` in `registry-system` namespace.
    - In the configmap, there're two json data: `whitelist-images`, `whitelist-namespaces`. Add an image's name to `whitelist-images` or a namespace's name to `whitelist-namespaces`. (Refer to the [example](./deploy/whitelist-configmap.yaml))  
      `CAUTION`: Multiple whitelist entries must be separated by a newline(\n)
    - For `whitelist-images`, wildcard for image name is supported.  
      e.g., if `whitelist-image` contains `registry-example.com/*`, then `registry-example.com/image-1` `registry-example.com/image-2` are treated as whitelisted.
    - For `whitelist-images`, host, tag, digest can be omitted. They will be treated as a wildcard.  
      e.g., `registry` in `whitelist-images` will treat `registry-1.com/registry:tag1` and `registry-2.com/registry:tag2` as whitelisted.

2. for user :

    - Default policy of image-validation-webhook is permitting pod creation with images from any registries.
    - You can restrict which registries to pull the images from: Use CRD named RegistySecurityPolicy & ClusterRegistrySecurityPolicy: Sample is
      ```yaml
      apiVersion: tmax.io/v1
      kind: RegistrySecurityPolicy
      metadata:
        name: sample-policy
        namespace: some-namespace
      spec:
        registries:
          - registry: core.harbor.domain.io
            notary: https://notary.harbor.domain.io
            cosignKeyRef: k8s://<namespace>/<cosign_key_secret>
            signer: ["<signer1>","<signer2>"]
            signCheck: true
      ---
      apiVersion: tmax.io/v1
      kind: ClusterRegistrySecurityPolicy
      metadata:
        name: sample-cluster-policy
      spec:
        registries:
          - registry: docker.io
            cosignKeyRef: k8s://<namespace>/<cosign_key_secret>
            signer: ["<signer1>"]
            signCheck: true
      ```
    - RegistrySecurityPolicy is a namespaced scope resource and you can add the trusted registries to `registries` list
    - ClusterRegistrySecurityPolicy is a cluster scope resource and works exactly same as RegistrySecurityPolicy in all namespaces
    - registries array consists of

        - Registry: Registry's url
        - Notary: Registry's corresponding notary server url
        - CosignKeyRef: The secret that includes pub/private key pair
        - Signer: A list of desired signers for the image that will be allowed to be distributed.
            - signer로 등록한 여러 서명자 리스트 중 하나라도 서명했다면 valid
        - Signcheck: If it is false, all images from this registry are allowed without checking their signature

3. Example flows of image validity check
    1. Image가 whitelist 목록에 포함된 경우 : VALID
    2. No Policy(Policy가 생성되지 않은 경우): VALID
    3. Policy가 존재 & image registry가 Policy에 포함되지 않은 경우 : INVALID
    4. Policy가 존재 & image registry가 Policy에 포함 & signCheck가 false인 경우 : VALID
    5. Policy가 존재 & image registry가 Policy에 포함 & signCheck가 true -> Notary, Cosign 순으로 서명 검사
      - Notary
        - Image가 Notary로 서명되었고 signer가 일치하는 경우 : VALID
        - Image가 Notary로 서명되었고 signer가 일치하지 않는 경우 -> Cosign으로 서명되었는지 검사
        - Image가 Notary로 서명되지 않은경우 -> Cosign으로 서명되었는지 검사
      - Cosign
        - Image가 Cosign으로 서명되었고 signer가 일치하는 경우 : VALID
        - Image가 Cosign으로 서명되었고 signer가 일치하지 않는 경우 : INVALID
        - Image가 Cosign으로 서명되지 않은경우 : INVALID
