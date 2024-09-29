<div align="center">

[![License](https://img.shields.io/badge/License-MIT-blue)](#license)
[![Go Report Card](https://goreportcard.com/badge/github.com/daytonaio/daytona-provider-gcp)](https://goreportcard.com/report/github.com/daytonaio/daytona-provider-gcp)
[![Issues - daytona](https://img.shields.io/github/issues/daytonaio/daytona-gcp-provider)](https://github.com/daytonaio/daytona-provider-gcp/issues)
![GitHub Release](https://img.shields.io/github/v/release/daytonaio/daytona-gcp-provider)

</div>


<h1 align="center">Daytona GCP Provider</h1>
<div align="center">
This repository is the home of the <a href="https://github.com/daytonaio/daytona">Daytona</a> GCP Provider.
</div>
</br>


<p align="center">
  <a href="https://github.com/daytonaio/daytona-provider-gcp/issues/new?assignees=&labels=bug&projects=&template=bug_report.md&title=%F0%9F%90%9B+Bug+Report%3A+">Report Bug</a>
    ·
  <a href="https://github.com/daytonaio/daytona-provider-gcp/issues/new?assignees=&labels=enhancement&projects=&template=feature_request.md&title=%F0%9F%9A%80+Feature%3A+">Request Feature</a>
    ·
  <a href="https://go.daytona.io/slack">Join Our Slack</a>
    ·
  <a href="https://x.com/Daytonaio">X</a>
</p>


The GCP Provider allows Daytona to create and manage workspace projects on Google Cloud Platform compute instances.

To use the GCP Provider for managing compute instances, you'll need to create a service account with the `Compute Admin` role. 
Download the service account key in JSON format and provide it to the GCP provider for authentication

Detailed instructions on create and configuring the service account can be found [here](https://cloud.google.com/iam/docs/service-accounts-create#console)

## Target Options

| Property                	        | Type     	    | Optional 	  | DefaultValue                	                                 | InputMasked 	   | DisabledPredicate 	 |
|----------------------------------|---------------|-------------|---------------------------------------------------------------|-----------------|---------------------|
| Zone         	                   | String   	    | true    	   | us-central1-a   	                                             | false       	   | 	                   |
| Machine Type                     | String   	    | true     	  | n1-standard-1                          	                      | false         	 | 	                   |
| Disk Type             	          | String      	 | true     	  | 	    pd-standard                                              | false       	   | 	                   |
| Disk Size                	       | Int 	         | true     	  | 20                                                            | false       	   |                     |
| VM Image                	        | String 	      | true     	  | projects/ubuntu-os-cloud/global/images/family/ubuntu-2204-lts | false       	   |                     |
| Credential File                	 | FilePath 	    | false     	 |                                                               | true       	    |                     |
| Project Id                	      | String 	      | false     	 |                                                               | true       	    |                     |

### Default Targets

The GCP Provider has no default targets. Before using the provider you must set the target using the daytona target set command.

## Code of Conduct

This project has adapted the Code of Conduct from the [Contributor Covenant](https://www.contributor-covenant.org/). For more information see the [Code of Conduct](CODE_OF_CONDUCT.md) or contact [codeofconduct@daytona.io.](mailto:codeofconduct@daytona.io) with any additional questions or comments.

## Contributing

The Daytona Docker Provider is Open Source under the [MIT License](LICENSE). If you would like to contribute to the software, you must:

1. Read the Developer Certificate of Origin Version 1.1 (https://developercertificate.org/)
2. Sign all commits to the Daytona Docker Provider project.

This ensures that users, distributors, and other contributors can rely on all the software related to Daytona being contributed under the terms of the [License](LICENSE). No contributions will be accepted without following this process.

Afterwards, navigate to the [contributing guide](CONTRIBUTING.md) to get started.

## Questions

For more information on how to use and develop Daytona, talk to us on
[Slack](https://go.daytona.io/slack).

