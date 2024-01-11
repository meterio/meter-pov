package builtin

var (
	// ----------------------------------
	// fork 11: upgrade USDC.eth, USDT.eth and WBTC.eth to add `permit` function
	// ----------------------------------
	// ERC20MinterBurnerPauserPermit: added `permit` function on ERC20MinterBurnerPauser
	ERC20MinterBurnerPauserPermit_ABI = convertABI(`[{"inputs":[{"internalType":"string","name":"_name","type":"string"},{"internalType":"string","name":"_symbol","type":"string"},{"internalType":"uint8","name":"decimals_","type":"uint8"}],"stateMutability":"nonpayable","type":"constructor"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"owner","type":"address"},{"indexed":true,"internalType":"address","name":"spender","type":"address"},{"indexed":false,"internalType":"uint256","name":"value","type":"uint256"}],"name":"Approval","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"address","name":"account","type":"address"}],"name":"Paused","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"bytes32","name":"role","type":"bytes32"},{"indexed":true,"internalType":"bytes32","name":"previousAdminRole","type":"bytes32"},{"indexed":true,"internalType":"bytes32","name":"newAdminRole","type":"bytes32"}],"name":"RoleAdminChanged","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"bytes32","name":"role","type":"bytes32"},{"indexed":true,"internalType":"address","name":"account","type":"address"},{"indexed":true,"internalType":"address","name":"sender","type":"address"}],"name":"RoleGranted","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"bytes32","name":"role","type":"bytes32"},{"indexed":true,"internalType":"address","name":"account","type":"address"},{"indexed":true,"internalType":"address","name":"sender","type":"address"}],"name":"RoleRevoked","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"from","type":"address"},{"indexed":true,"internalType":"address","name":"to","type":"address"},{"indexed":false,"internalType":"uint256","name":"value","type":"uint256"}],"name":"Transfer","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"address","name":"account","type":"address"}],"name":"Unpaused","type":"event"},{"inputs":[],"name":"DEFAULT_ADMIN_ROLE","outputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"DOMAIN_SEPARATOR","outputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"MINTER_ROLE","outputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"PAUSER_ROLE","outputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"_CACHED_CHAIN_ID","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"_CACHED_DOMAIN_SEPARATOR","outputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"_CACHED_THIS","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"_CONST_PERMIT_TYPEHASH","outputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"_HASHED_NAME","outputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"_HASHED_VERSION","outputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"_TYPE_HASH","outputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"owner","type":"address"},{"internalType":"address","name":"spender","type":"address"}],"name":"allowance","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"spender","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"approve","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"account","type":"address"}],"name":"balanceOf","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"burn","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"account","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"burnFrom","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"decimals","outputs":[{"internalType":"uint8","name":"","type":"uint8"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"spender","type":"address"},{"internalType":"uint256","name":"subtractedValue","type":"uint256"}],"name":"decreaseAllowance","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"bytes32","name":"role","type":"bytes32"}],"name":"getRoleAdmin","outputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"bytes32","name":"role","type":"bytes32"},{"internalType":"uint256","name":"index","type":"uint256"}],"name":"getRoleMember","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"bytes32","name":"role","type":"bytes32"}],"name":"getRoleMemberCount","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"bytes32","name":"role","type":"bytes32"},{"internalType":"address","name":"account","type":"address"}],"name":"grantRole","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"bytes32","name":"role","type":"bytes32"},{"internalType":"address","name":"account","type":"address"}],"name":"hasRole","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"spender","type":"address"},{"internalType":"uint256","name":"addedValue","type":"uint256"}],"name":"increaseAllowance","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"mint","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"name","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"owner","type":"address"}],"name":"nonces","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"pause","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"paused","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"owner","type":"address"},{"internalType":"address","name":"spender","type":"address"},{"internalType":"uint256","name":"value","type":"uint256"},{"internalType":"uint256","name":"deadline","type":"uint256"},{"internalType":"bytes","name":"signature","type":"bytes"}],"name":"permit","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"bytes32","name":"role","type":"bytes32"},{"internalType":"address","name":"account","type":"address"}],"name":"renounceRole","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"bytes32","name":"role","type":"bytes32"},{"internalType":"address","name":"account","type":"address"}],"name":"revokeRole","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"symbol","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"totalSupply","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"recipient","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"transfer","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"sender","type":"address"},{"internalType":"address","name":"recipient","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"transferFrom","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"unpause","outputs":[],"stateMutability":"nonpayable","type":"function"}]
`)
	ERC20MinterBurnerPauserPermit_DeployedBytecode = convertBytecode("0x608060405234801561001057600080fd5b50600436106102275760003560e01c8063712ac56d11610130578063a457c2d7116100b8578063d53913931161007c578063d5391393146106c6578063d547741f146106ce578063da28b527146106fa578063dd62ed3e14610702578063e63ab1e91461073057610227565b8063a457c2d714610641578063a9059cbb1461066d578063a9e91e5414610699578063ca15c873146106a1578063caac6c82146106be57610227565b80639010d07c116100ff5780639010d07c146104fb57806391d148541461053a57806395d89b41146105665780639fd5a6cf1461056e578063a217fddf1461063957610227565b8063712ac56d1461049957806379cc6790146104a15780637ecebe00146104cd5780638456cb59146104f357610227565b80633644e515116101b357806340c10f191161018257806340c10f191461041a57806342966c68146104465780635c975abb146104635780635d2dab0b1461046b57806370a082311461047357610227565b80633644e515146103b257806336568abe146103ba57806339509351146103e65780633f4ba83a1461041257610227565b8063248a9ca3116101fa578063248a9ca3146103395780632b437d48146103565780632f2ff15d1461035e578063313ce5671461038c57806334f9d406146103aa57610227565b806306fdde031461022c578063095ea7b3146102a957806318160ddd146102e957806323b872dd14610303575b600080fd5b610234610738565b6040805160208082528351818301528351919283929083019185019080838360005b8381101561026e578181015183820152602001610256565b50505050905090810190601f16801561029b5780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b6102d5600480360360408110156102bf57600080fd5b506001600160a01b0381351690602001356107cf565b604080519115158252519081900360200190f35b6102f16107ed565b60408051918252519081900360200190f35b6102d56004803603606081101561031957600080fd5b506001600160a01b038135811691602081013590911690604001356107f3565b6102f16004803603602081101561034f57600080fd5b503561087a565b6102f161088f565b61038a6004803603604081101561037457600080fd5b50803590602001356001600160a01b0316610894565b005b610394610900565b6040805160ff9092168252519081900360200190f35b6102f1610909565b6102f161092d565b61038a600480360360408110156103d057600080fd5b50803590602001356001600160a01b031661093c565b6102d5600480360360408110156103fc57600080fd5b506001600160a01b03813516906020013561099d565b61038a6109eb565b61038a6004803603604081101561043057600080fd5b506001600160a01b038135169060200135610a5c565b61038a6004803603602081101561045c57600080fd5b5035610acd565b6102d5610ae1565b6102f1610aef565b6102f16004803603602081101561048957600080fd5b50356001600160a01b0316610b13565b6102f1610b2e565b61038a600480360360408110156104b757600080fd5b506001600160a01b038135169060200135610b52565b6102f1600480360360208110156104e357600080fd5b50356001600160a01b0316610bac565b61038a610bcd565b61051e6004803603604081101561051157600080fd5b5080359060200135610c3c565b604080516001600160a01b039092168252519081900360200190f35b6102d56004803603604081101561055057600080fd5b50803590602001356001600160a01b0316610c5b565b610234610c73565b61038a600480360360a081101561058457600080fd5b6001600160a01b03823581169260208101359091169160408201359160608101359181019060a0810160808201356401000000008111156105c457600080fd5b8201836020820111156105d657600080fd5b803590602001918460018302840111640100000000831117156105f857600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600092019190915250929550610cd4945050505050565b6102f1610e4c565b6102d56004803603604081101561065757600080fd5b506001600160a01b038135169060200135610e51565b6102d56004803603604081101561068357600080fd5b506001600160a01b038135169060200135610eb9565b6102f1610ecd565b6102f1600480360360208110156106b757600080fd5b5035610ef1565b6102f1610f08565b6102f1610f2c565b61038a600480360360408110156106e457600080fd5b50803590602001356001600160a01b0316610f50565b61051e610fa9565b6102f16004803603604081101561071857600080fd5b506001600160a01b0381358116916020013516610fc1565b6102f1610fec565b60048054604080516020601f60026000196101006001881615020190951694909404938401819004810282018101909252828152606093909290918301828280156107c45780601f10610799576101008083540402835291602001916107c4565b820191906000526020600020905b8154815290600101906020018083116107a757829003601f168201915b505050505090505b90565b60006107e36107dc611025565b8484611029565b5060015b92915050565b60035490565b6000610800848484611115565b6108708461080c611025565b61086b85604051806060016040528060288152602001611f50602891396001600160a01b038a1660009081526002602052604081209061084a611025565b6001600160a01b031681526020810191909152604001600020549190611272565b611029565b5060019392505050565b60009081526020819052604090206002015490565b605281565b6000828152602081905260409020600201546108b7906108b2611025565b610c5b565b6108f25760405162461bcd60e51b815260040180806020018281038252602f815260200180611e2c602f913960400191505060405180910390fd5b6108fc8282611309565b5050565b60065460ff1690565b7f6e71edae12b1b97f4d1f60370fef10105fa2faae0126114a169c64845d6126c981565b6000610937611372565b905090565b610944611025565b6001600160a01b0316816001600160a01b0316146109935760405162461bcd60e51b815260040180806020018281038252602f815260200180612098602f913960400191505060405180910390fd5b6108fc828261143c565b60006107e36109aa611025565b8461086b85600260006109bb611025565b6001600160a01b03908116825260208083019390935260409182016000908120918c1681529252902054906114a5565b610a177f65d7a28e3265b37a6474929f336521b332c1681b933f6cb9f3376673440d862a6108b2611025565b610a525760405162461bcd60e51b8152600401808060200182810382526039815260200180611e7d6039913960400191505060405180910390fd5b610a5a6114ff565b565b610a887f9f2df0fed2c77648de5860a4cc508cd0818c85b8b8a1ab4ceeef8d981c8956a66108b2611025565b610ac35760405162461bcd60e51b8152600401808060200182810382526036815260200180611f786036913960400191505060405180910390fd5b6108fc82826115a0565b610ade610ad8611025565b82611692565b50565b600654610100900460ff1690565b7f8b73c3c69bb8fe3d512ecc4cf759cc79239f7b179b0ffacaa9a75d522b39400f81565b6001600160a01b031660009081526001602052604090205490565b7fe8245ed8f93ccf147bc8972ab6d5f8278ed994d647700daaa5b7ebde9370aa6f81565b6000610b8982604051806060016040528060248152602001611fae60249139610b8286610b7d611025565b610fc1565b9190611272565b9050610b9d83610b97611025565b83611029565b610ba78383611692565b505050565b6001600160a01b03811660009081526007602052604081206107e79061178e565b610bf97f65d7a28e3265b37a6474929f336521b332c1681b933f6cb9f3376673440d862a6108b2611025565b610c345760405162461bcd60e51b815260040180806020018281038252603781526020018061203c6037913960400191505060405180910390fd5b610a5a611792565b6000828152602081905260408120610c549083611817565b9392505050565b6000828152602081905260408120610c549083611823565b60058054604080516020601f60026000196101006001881615020190951694909404938401819004810282018101909252828152606093909290918301828280156107c45780601f10610799576101008083540402835291602001916107c4565b81421115610d29576040805162461bcd60e51b815260206004820152601d60248201527f45524332305065726d69743a206578706972656420646561646c696e65000000604482015290519081900360640190fd5b60007f6e71edae12b1b97f4d1f60370fef10105fa2faae0126114a169c64845d6126c9868686610d5883611838565b8760405160200180878152602001866001600160a01b03168152602001856001600160a01b0316815260200184815260200183815260200182815260200196505050505050506040516020818303038152906040528051906020012090506000610dc18261186a565b90506000610dcf828561187d565b9050876001600160a01b0316816001600160a01b031614610e37576040805162461bcd60e51b815260206004820152601e60248201527f45524332305065726d69743a20696e76616c6964207369676e61747572650000604482015290519081900360640190fd5b610e42888888611029565b5050505050505050565b600081565b60006107e3610e5e611025565b8461086b856040518060600160405280602581526020016120736025913960026000610e88611025565b6001600160a01b03908116825260208083019390935260409182016000908120918d16815292529020549190611272565b60006107e3610ec6611025565b8484611115565b7f5d0a451daeda5bd9f4095b6c09da34bdf3b91f4b8b8f60e3dd42d9d0d1ed158481565b60008181526020819052604081206107e7906118a1565b7f5e422fe6c718eb4b17d6b107d0b546cf8641493faaaddb68e483cd686c1d756c81565b7f9f2df0fed2c77648de5860a4cc508cd0818c85b8b8a1ab4ceeef8d981c8956a681565b600082815260208190526040902060020154610f6e906108b2611025565b6109935760405162461bcd60e51b8152600401808060200182810382526030815260200180611f206030913960400191505060405180910390fd5b73228ebbee999c6a7ad74a6130e81b12f9fe237ba381565b6001600160a01b03918216600090815260026020908152604080832093909416825291909152205490565b7f65d7a28e3265b37a6474929f336521b332c1681b933f6cb9f3376673440d862a81565b6000610c54836001600160a01b0384166118ac565b3390565b6001600160a01b03831661106e5760405162461bcd60e51b81526004018080602001828103825260248152602001806120186024913960400191505060405180910390fd5b6001600160a01b0382166110b35760405162461bcd60e51b8152600401808060200182810382526022815260200180611eb66022913960400191505060405180910390fd5b6001600160a01b03808416600081815260026020908152604080832094871680845294825291829020859055815185815291517f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b9259281900390910190a3505050565b6001600160a01b03831661115a5760405162461bcd60e51b8152600401808060200182810382526025815260200180611ff36025913960400191505060405180910390fd5b6001600160a01b03821661119f5760405162461bcd60e51b8152600401808060200182810382526023815260200180611e096023913960400191505060405180910390fd5b6111aa8383836118f6565b6111e781604051806060016040528060268152602001611ed8602691396001600160a01b0386166000908152600160205260409020549190611272565b6001600160a01b03808516600090815260016020526040808220939093559084168152205461121690826114a5565b6001600160a01b0380841660008181526001602090815260409182902094909455805185815290519193928716927fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef92918290030190a3505050565b600081848411156113015760405162461bcd60e51b81526004018080602001828103825283818151815260200191508051906020019080838360005b838110156112c65781810151838201526020016112ae565b50505050905090810190601f1680156112f35780820380516001836020036101000a031916815260200191505b509250505060405180910390fd5b505050900390565b60008281526020819052604090206113219082611010565b156108fc5761132e611025565b6001600160a01b0316816001600160a01b0316837f2f8788117e7eff1d82e926ec794901d17c78024a50270940304540a733656f0d60405160405180910390a45050565b60003073228ebbee999c6a7ad74a6130e81b12f9fe237ba314801561139e5750605261139c611901565b145b156113ca57507f5d0a451daeda5bd9f4095b6c09da34bdf3b91f4b8b8f60e3dd42d9d0d1ed15846107cc565b6114357f8b73c3c69bb8fe3d512ecc4cf759cc79239f7b179b0ffacaa9a75d522b39400f7f5e422fe6c718eb4b17d6b107d0b546cf8641493faaaddb68e483cd686c1d756c7fe8245ed8f93ccf147bc8972ab6d5f8278ed994d647700daaa5b7ebde9370aa6f611905565b90506107cc565b60008281526020819052604090206114549082611967565b156108fc57611461611025565b6001600160a01b0316816001600160a01b0316837ff6391f5c32d9c69d2a47ea670b442974b53935d1edc7fd64eb21e047a839171b60405160405180910390a45050565b600082820183811015610c54576040805162461bcd60e51b815260206004820152601b60248201527f536166654d6174683a206164646974696f6e206f766572666c6f770000000000604482015290519081900360640190fd5b611507610ae1565b61154f576040805162461bcd60e51b815260206004820152601460248201527314185d5cd8589b194e881b9bdd081c185d5cd95960621b604482015290519081900360640190fd5b6006805461ff00191690557f5db9ee0a495bf2e6ff9c91a7834c1ba4fdd244a5e8aa4e537bd38aeae4b073aa611583611025565b604080516001600160a01b039092168252519081900360200190a1565b6001600160a01b0382166115fb576040805162461bcd60e51b815260206004820152601f60248201527f45524332303a206d696e7420746f20746865207a65726f206164647265737300604482015290519081900360640190fd5b611607600083836118f6565b60035461161490826114a5565b6003556001600160a01b03821660009081526001602052604090205461163a90826114a5565b6001600160a01b03831660008181526001602090815260408083209490945583518581529351929391927fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef9281900390910190a35050565b6001600160a01b0382166116d75760405162461bcd60e51b8152600401808060200182810382526021815260200180611fd26021913960400191505060405180910390fd5b6116e3826000836118f6565b61172081604051806060016040528060228152602001611e5b602291396001600160a01b0385166000908152600160205260409020549190611272565b6001600160a01b038316600090815260016020526040902055600354611746908261197c565b6003556040805182815290516000916001600160a01b038516917fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef9181900360200190a35050565b5490565b61179a610ae1565b156117df576040805162461bcd60e51b815260206004820152601060248201526f14185d5cd8589b194e881c185d5cd95960821b604482015290519081900360640190fd5b6006805461ff0019166101001790557f62e78cea01bee320cd4e420270b5ea74000d11b0c9f74754ebdbfc544b05a258611583611025565b6000610c5483836119d9565b6000610c54836001600160a01b038416611a3d565b6001600160a01b03811660009081526007602052604081206118598161178e565b915061186481611a55565b50919050565b60006107e7611877611372565b83611a5e565b600080600061188c8585611a99565b9150915061189981611adf565b509392505050565b60006107e78261178e565b60006118b88383611a3d565b6118ee575081546001818101845560008481526020808220909301849055845484825282860190935260409020919091556107e7565b5060006107e7565b610ba7838383611c05565b4690565b6000838383611912611901565b3060405160200180868152602001858152602001848152602001838152602001826001600160a01b03168152602001955050505050506040516020818303038152906040528051906020012090509392505050565b6000610c54836001600160a01b038416611c54565b6000828211156119d3576040805162461bcd60e51b815260206004820152601e60248201527f536166654d6174683a207375627472616374696f6e206f766572666c6f770000604482015290519081900360640190fd5b50900390565b81546000908210611a1b5760405162461bcd60e51b8152600401808060200182810382526022815260200180611de76022913960400191505060405180910390fd5b826000018281548110611a2a57fe5b9060005260206000200154905092915050565b60009081526001919091016020526040902054151590565b80546001019055565b6040805161190160f01b6020808301919091526022820194909452604280820193909352815180820390930183526062019052805191012090565b600080825160411415611ad05760208301516040840151606085015160001a611ac487828585611d1a565b94509450505050611ad8565b506000905060025b9250929050565b6000816004811115611aed57fe5b1415611af857610ade565b6001816004811115611b0657fe5b1415611b59576040805162461bcd60e51b815260206004820152601860248201527f45434453413a20696e76616c6964207369676e61747572650000000000000000604482015290519081900360640190fd5b6002816004811115611b6757fe5b1415611bba576040805162461bcd60e51b815260206004820152601f60248201527f45434453413a20696e76616c6964207369676e6174757265206c656e67746800604482015290519081900360640190fd5b6003816004811115611bc857fe5b1415610ade5760405162461bcd60e51b8152600401808060200182810382526022815260200180611efe6022913960400191505060405180910390fd5b611c10838383610ba7565b611c18610ae1565b15610ba75760405162461bcd60e51b815260040180806020018281038252602a8152602001806120c7602a913960400191505060405180910390fd5b60008181526001830160205260408120548015611d105783546000198083019190810190600090879083908110611c8757fe5b9060005260206000200154905080876000018481548110611ca457fe5b600091825260208083209091019290925582815260018981019092526040902090840190558654879080611cd457fe5b600190038181906000526020600020016000905590558660010160008781526020019081526020016000206000905560019450505050506107e7565b60009150506107e7565b6000807f7fffffffffffffffffffffffffffffff5d576e7357a4501ddfe92f46681b20a0831115611d515750600090506003611ddd565b600060018787878760405160008152602001604052604051808581526020018460ff1681526020018381526020018281526020019450505050506020604051602081039080840390855afa158015611dad573d6000803e3d6000fd5b5050604051601f1901519150506001600160a01b038116611dd657600060019250925050611ddd565b9150600090505b9450949250505056fe456e756d657261626c655365743a20696e646578206f7574206f6620626f756e647345524332303a207472616e7366657220746f20746865207a65726f2061646472657373416363657373436f6e74726f6c3a2073656e646572206d75737420626520616e2061646d696e20746f206772616e7445524332303a206275726e20616d6f756e7420657863656564732062616c616e636545524332305072657365744d696e7465725061757365723a206d75737420686176652070617573657220726f6c6520746f20756e706175736545524332303a20617070726f766520746f20746865207a65726f206164647265737345524332303a207472616e7366657220616d6f756e7420657863656564732062616c616e636545434453413a20696e76616c6964207369676e6174757265202773272076616c7565416363657373436f6e74726f6c3a2073656e646572206d75737420626520616e2061646d696e20746f207265766f6b6545524332303a207472616e7366657220616d6f756e74206578636565647320616c6c6f77616e636545524332305072657365744d696e7465725061757365723a206d7573742068617665206d696e74657220726f6c6520746f206d696e7445524332303a206275726e20616d6f756e74206578636565647320616c6c6f77616e636545524332303a206275726e2066726f6d20746865207a65726f206164647265737345524332303a207472616e736665722066726f6d20746865207a65726f206164647265737345524332303a20617070726f76652066726f6d20746865207a65726f206164647265737345524332305072657365744d696e7465725061757365723a206d75737420686176652070617573657220726f6c6520746f20706175736545524332303a2064656372656173656420616c6c6f77616e63652062656c6f77207a65726f416363657373436f6e74726f6c3a2063616e206f6e6c792072656e6f756e636520726f6c657320666f722073656c6645524332305061757361626c653a20746f6b656e207472616e73666572207768696c6520706175736564a2646970667358221220c02119084d35bbf9feb9fe0028ad6174b270b3d055220da0a97645b48cb3c1ef64736f6c63430007000033")
)
