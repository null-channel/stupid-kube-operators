# e-mail operator

Have you wanted to use kubernetes to send e-mail? well now you can simply and easilly!

*** NOTE: This operator should not be used in production, in not production or proably at all! it's for fun only. ***

This project was created for educational purposes. I will  try to explain it as best as possible here.

## Creating the operator
The operator was created using kubebuilder with the following command:

`kubebuilder init --domain thenullchannel.dev --repo github.com/null-channel/stupid-kube-operators/email`


The api was then created using

`kubebuilder create api --group nullemail --version v1 --kind Email`
