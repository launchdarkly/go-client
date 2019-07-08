LaunchDarkly SDK for Go
===========================

[![Circle CI](https://circleci.com/gh/launchdarkly/go-client.svg?style=svg)](https://circleci.com/gh/launchdarkly/go-client)

Important note
--------------

As mentioned in the [repository changelog](https://github.com/launchdarkly/go-client/blob/v4/CHANGELOG.md#note-on-future-releases), the `go-client` project has been renamed to `go-server-sdk`. All future releases will be made from the [new repository](https://github.com/launchdarkly/go-server-sdk). Please consider updating your import paths and filing potential requests in that repository's [issue tracker](https://github.com/launchdarkly/go-server-sdk/issues).

Go runtime compatibility
-------------------------

This version of the LaunchDarkly SDK has been tested with Go 1.8 through 1.10.

Quick setup
-----------

1. Install the SDK with the `go` tool:

```bash
go get gopkg.in/launchdarkly/go-client.v4
```

2. Import the LaunchDarkly client:

```go
import ld "gopkg.in/launchdarkly/go-client.v4"
```

3. Create a new LDClient with your SDK key:

```go
ldClient, err := ld.MakeClient("YOUR_SDK_KEY", 5*time.Second)
if err != nil {
    log.Fatalf("Error creating launch darkly client: %s", err)
}
defer ldClient.Close()
```

If you are reusing a global instance you probably want to not  `defer ldClient.Close()` but instead close it when the application exits.


HTTPS proxy
------------
Go's standard HTTP library provides built-in support for the use of an HTTPS proxy. If the HTTPS_PROXY environment variable is present then the SDK will proxy all network requests through the URL provided.

How to set the HTTPS_PROXY environment variable on Mac/Linux systems:
```
export HTTPS_PROXY=https://web-proxy.domain.com:8080
```


How to set the HTTPS_PROXY environment variable on Windows systems:
```
set HTTPS_PROXY=https://web-proxy.domain.com:8080
```


If your proxy requires authentication then you can prefix the URN with your login information:
```
export HTTPS_PROXY=http://user:pass@web-proxy.domain.com:8080
```
or
```
set HTTPS_PROXY=http://user:pass@web-proxy.domain.com:8080
```


Your first feature flag
-----------------------

1. Create a new feature flag on your [dashboard](https://app.launchdarkly.com)
2. In your application code, use the feature's key to check whether the flag is on for each user:

```go
key := "user@test.com"
showFeature, _ := ldClient.BoolVariation("your.flag.key", ld.User{Key: &key}, false)
if showFeature {
    // application code to show the feature
} else {
    // the code to run if the feature is off
}
```

Database integrations
---------------------

Feature flag data can be kept in a persistent store using Redis, Consul, or DynamoDB. These adapters are implemented in the subpackages `redis`, `ldconsul`, and `lddynamodb`; to use them, call the `New...FeatureStore` function provided by the subpackage, and put the returned object in the `FeatureStore` field of your client configuration. See the subpackages and the [SDK reference guide](https://docs.launchdarkly.com/v2.0/docs/using-a-persistent-feature-store) for more information.

Using flag data from a file
---------------------------

For testing purposes, the SDK can be made to read feature flag state from a file or files instead of connecting to LaunchDarkly. See [`ldfiledata`](https://godoc.org/github.com/launchdarkly/go-client/ldfiledata) and [`ldfilewatch`](https://godoc.org/github.com/launchdarkly/go-client/ldfilewatch) for more details.

Learn more
-----------

Check out our [documentation](http://docs.launchdarkly.com) for in-depth instructions on configuring and using LaunchDarkly. You can also head straight to the [complete reference guide for this SDK](http://docs.launchdarkly.com/docs/go-sdk-reference) and the [API reference](https://godoc.org/github.com/launchdarkly/go-client).

Testing
-------

We run integration tests for all our SDKs using a centralized test harness. This approach gives us the ability to test for consistency across SDKs, as well as test networking behavior in a long-running application. These tests cover each method in the SDK, and verify that event sending, flag evaluation, stream reconnection, and other aspects of the SDK all behave correctly.

Contributing
------------

We encourage pull-requests and other contributions from the community. We've also published an [SDK contributor's guide](http://docs.launchdarkly.com/docs/sdk-contributors-guide) that provides a detailed explanation of how our SDKs work.

About LaunchDarkly
-----------

* LaunchDarkly is a continuous delivery platform that provides feature flags as a service and allows developers to iterate quickly and safely. We allow you to easily flag your features and manage them from the LaunchDarkly dashboard.  With LaunchDarkly, you can:
    * Roll out a new feature to a subset of your users (like a group of users who opt-in to a beta tester group), gathering feedback and bug reports from real-world use cases.
    * Gradually roll out a feature to an increasing percentage of users, and track the effect that the feature has on key metrics (for instance, how likely is a user to complete a purchase if they have feature A versus feature B?).
    * Turn off a feature that you realize is causing performance problems in production, without needing to re-deploy, or even restart the application with a changed configuration file.
    * Grant access to certain features based on user attributes, like payment plan (eg: users on the ‘gold’ plan get access to more features than users in the ‘silver’ plan). Disable parts of your application to facilitate maintenance, without taking everything offline.
* LaunchDarkly provides feature flag SDKs for a wide variety of languages and technologies. Check out [our documentation](https://docs.launchdarkly.com/docs) for a complete list.
* Explore LaunchDarkly
    * [launchdarkly.com](https://www.launchdarkly.com/ "LaunchDarkly Main Website") for more information
    * [docs.launchdarkly.com](https://docs.launchdarkly.com/  "LaunchDarkly Documentation") for our documentation and SDK reference guides
    * [apidocs.launchdarkly.com](https://apidocs.launchdarkly.com/  "LaunchDarkly API Documentation") for our API documentation
    * [blog.launchdarkly.com](https://blog.launchdarkly.com/  "LaunchDarkly Blog Documentation") for the latest product updates
    * [Feature Flagging Guide](https://github.com/launchdarkly/featureflags/  "Feature Flagging Guide") for best practices and strategies
