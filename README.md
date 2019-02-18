# Installation

Setting up ecobee2prom is complicated by the need to authenticate with the Ecobee API service. The first time, this requires some manual steps, however ecobee2prom will subsequently manage its own authentication, given somewhere to store passwords.

Running the program locally is straightforward: set --cache_file to some persistent path and follow ecobee2prom's interactive prompts the first time you run it.

The following instructions are for using the Docker image.

## Step 1: create a volume to store API passwords

`docker volume create ecobee_data`

## Step 2: run ecobee2prom once, interactively, to respond to its prompts

`docker run -v ecobee_data:/db -p 8080:8080 -it dichro/ecobee2prom:latest`

This will print nothing until...

## Step 3: request a metric update

Since ecobee2prom is a proxy, rather than a typical caching exporter, you'll have to point your browser to `http://localhost:8080/metrics` to trigger a metric fetch. This will hang in the browser, until you...

## Step 4: authorize ecobee2prom at ecobee.com

Back in your Docker window from step 2, ecobee2prom should now have printed something like:

> Pin is "ig7j"

> Press &lt;enter> after authorizing it on https://www.ecobee.com/consumerportal in the menu under 'My Apps'

Authorize the app via the Ecobee website, which as of 2019-02-18 can be found at `https://www.ecobee.com/consumerportal/#/my-apps`. Click `Add Application`, enter the Pin that ecobee2prom printed above, and confirm the authorization.

Then press enter in your Docker window from step 2 to continue.

Any errors here may be due to ecobee2prom being unable to write passwords into the `ecobee_data` volume, which is required for anything else to work.

## Step 5: confirm results.

Return to your browser window from step 3, which should now be displaying Prometheus metrics. ecobee_fetch_time should have measured the total elapsed time that it took you to complete step 4, and there should be a number of other ecobee_* metrics for your thermostat.

## Step 6: run non-interactively

^C your docker window, and re-run it without the `-it` flag:

`docker run -v ecobee_data:/db -p 8080:8080 dichro/ecobee2prom:latest`

Reload your browser window from step 3 to fetch fresh metrics. You should see that ecobee_fetch_time is now much faster, on the order of a second or less, as it's reusing the passwords that it has already saved.