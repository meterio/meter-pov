// Abstract contract for the full ERC 20 Token standard
// https://github.com/ethereum/EIPs/issues/20

pragma solidity ^0.4.16;

contract owned {
    address public owner;

    function owned(address _owner) public {
        owner = _owner;
    }

    modifier onlyOwner {
        require(msg.sender == owner);
        _;
    }

    function transferOwnership(address newOwner) onlyOwner public {
        owner = newOwner;
    }
}

contract safeMath {
    function safeMul(uint a, uint b) internal constant returns (uint256) {
        uint c = a * b;
        assert(a == 0 || c / a == b);
        return c;
    }

    function safeDiv(uint a, uint b) internal constant returns (uint256) {
        uint c = a / b;
        return c;
    }

    function safeSub(uint a, uint b) internal constant returns (uint256) {
        assert(b <= a);
        return a - b;
    }

    function safeAdd(uint a, uint b) internal constant returns (uint256) {
        uint c = a + b;
        assert(c >= a);
        return c;
    }

    function max64(uint64 a, uint64 b) internal constant returns (uint64) {
        return a >= b ? a : b;
    }

    function min64(uint64 a, uint64 b) internal constant returns (uint64) {
        return a < b ? a : b;
    }

    function max256(uint256 a, uint256 b) internal constant returns (uint256) {
        return a >= b ? a : b;
    }

    function min256(uint256 a, uint256 b) internal constant returns (uint256) {
        return a < b ? a : b;
    }
}

/* This is a slight change to the ERC20 base standard.
    function totalSupply() constant returns (uint256 supply);
    is replaced with:
    uint256 public totalSupply;
    This automatically creates a getter function for the totalSupply.
    This is moved to the base contract since public getter functions are not
    currently recognised as an implementation of the matching abstract
    function by the compiler.
    */

//interface tokenRecipient { function receiveApproval(address _from, uint256 _value, address _token, bytes _extraData) public; }

contract TokenERC20 {
    // Public variables of the token
    string public name;
    string public symbol;
    uint8 public decimals;
    // 18 decimals is the strongly suggested default, avoid changing it
    uint256 public totalSupply;

    // This creates an array with all balances
    mapping (address => uint256) balanceOf;
    mapping (address => mapping (address => uint256)) allowance;

    // This generates a public event on the blockchain that will notify clients
    event Transfer(address indexed from, address indexed to, uint256 value);

    // This notifies clients about the amount burnt
    event Burn(address indexed from, uint256 value);

    // This generates a public event of approve.
    event Approval(address indexed _owner, address indexed _spender, uint256 _value);

    event Balance(address target, uint256 value, string reason);
 
    /**
     * Constructor function
     *
     * Initializes contract with initial supply tokens to the creator of the contract
     */
    function TokenERC20(
        address initialOwner,
        uint256 initialSupply,
        string tokenName,
        string tokenSymbol,
        uint8  tokenDecimals
    ) public {
        totalSupply = initialSupply * 10 ** uint256(decimals);  // Update total supply with the decimal amount
        balanceOf[initialOwner] = totalSupply;                // Give the creator all initial tokens
        name = tokenName;                                   // Set the name for display purposes
        symbol = tokenSymbol;                               // Set the symbol for display purposes
        decimals = tokenDecimals;                           // normally 18
        
    }

    /**
     * Internal transfer, only can be called by this contract
     */
    function _transfer(address _from, address _to, uint _value) internal {
        // Prevent transfer to 0x0 address. Use burn() instead
        require(_to != 0x0);
        // Check if the sender has enough
        require(balanceOf[_from] >= _value);
        // Check for overflows
        require(balanceOf[_to] + _value > balanceOf[_to]);
        // Save this for an assertion in the future
        uint previousBalances = balanceOf[_from] + balanceOf[_to];
        // Subtract from the sender
        balanceOf[_from] -= _value;
        // Add the same to the recipient
        balanceOf[_to] += _value;
        Transfer(_from, _to, _value);
        // Asserts are used to use static analysis to find bugs in your code. They should never fail
        assert(balanceOf[_from] + balanceOf[_to] == previousBalances);
    }

    /**
     * Transfer tokens
     *
     * Send `_value` tokens to `_to` from your account
     *
     * @param _to The address of the recipient
     * @param _value the amount to send
     */
    function transfer(address _to, uint256 _value) public {
        _transfer(msg.sender, _to, _value);
    }

    /**
     * Transfer tokens from other address
     *
     * Send `_value` tokens to `_to` in behalf of `_from`
     *
     * @param _from The address of the sender
     * @param _to The address of the recipient
     * @param _value the amount to send
     */
    function transferFrom(address _from, address _to, uint256 _value) public returns (bool success) {
        require(_value <= allowance[_from][msg.sender]);     // Check allowance
        allowance[_from][msg.sender] -= _value;
        _transfer(_from, _to, _value);
        return true;
    }

    /**
     * Set allowance for other address
     *
     * Allows `_spender` to spend no more than `_value` tokens in your behalf
     *
     * @param _spender The address authorized to spend
     * @param _value the max amount they can spend
     */
    function approve(address _spender, uint256 _value) public
        returns (bool success) {
        allowance[msg.sender][_spender] = _value;
        Approval(msg.sender, _spender, _value);
        return true;
    }

    /**
     * Set allowance for other address and notify
     *
     * Allows `_spender` to spend no more than `_value` tokens in your behalf, and then ping the contract about it
     *
     * @param _spender The address authorized to spend
     * @param _value the max amount they can spend
     * @param _extraData some extra information to send to the approved contract
     */

    function approveAndCall(address _spender, uint256 _value, bytes _extraData)
        public
        returns (bool success) {
        RecipientSuccess spender = RecipientSuccess(_spender);
        if (approve(_spender, _value)) {
            spender.receiveApproval(msg.sender, _value, this, _extraData);
            return true;
        }
    }

    /**
     * Destroy tokens
     *
     * Remove `_value` tokens from the system irreversibly
     *
     * @param _value the amount of money to burn
     */
    function burn(uint256 _value) public returns (bool success) {
        require(balanceOf[msg.sender] >= _value);   // Check if the sender has enough
        balanceOf[msg.sender] -= _value;            // Subtract from the sender
        totalSupply -= _value;                      // Updates totalSupply
        Burn(msg.sender, _value);
        return true;
    }

    /**
     * Destroy tokens from other ccount
     *
     * Remove `_value` tokens from the system irreversibly on behalf of `_from`.
     *
     * @param _from the address of the sender
     * @param _value the amount of money to burn
     */
    function burnFrom(address _from, uint256 _value) public returns (bool success) {
        require(balanceOf[_from] >= _value);                // Check if the targeted balance is enough
        require(_value <= allowance[_from][msg.sender]);    // Check allowance
        balanceOf[_from] -= _value;                         // Subtract from the targeted balance
        allowance[_from][msg.sender] -= _value;             // Subtract from the sender's allowance
        totalSupply -= _value;                              // Update totalSupply
        Burn(_from, _value);
        return true;
    }
}

/******************************************
 * This is an example contract that helps test the functionality of the 
 * approveAndCall() functionality of HumanStandardToken.sol.
 * This one assumes successful receival of approval.
 ********************************************/
contract RecipientSuccess {
  /* A Generic receiving function for contracts that accept tokens */
  address public from;
  uint256 public value;
  address public tokenContract;
  bytes public extraData;

  event ReceivedApproval(uint256 _value);

  function receiveApproval(address _from, uint256 _value, address _tokenContract, bytes _extraData) {
    from = _from;
    value = _value;
    tokenContract = _tokenContract;
    extraData = _extraData;
    ReceivedApproval(_value);
  }
}

//contract RecipientThrow {
//}


/*

1) Initial Finite Supply (upon creation one specifies how much is minted).
2) In the absence of a token registry: Optional Decimal, Symbol & Name.
3) Optional approveAndCall() functionality to notify a contract if an approval() has occurred.

.*/
contract SAMPLEToken is owned, TokenERC20 {
            /* Public variables of the token */

    /*
    NOTE:
    The following variables are OPTIONAL vanities. One does not have to include them.
    They allow one to customise the token contract & in no way influences the core functionality.
    Some wallets/interfaces might not even bother to look at this information.
    */

    string public name;                   //fancy name: eg ChainWare Token
    uint8 public decimals;                //How many decimals to show. 
    string public symbol;                 //An identifier: eg SBX
    string public version = 'SAMPLE0.1';       //human 0.1 standard. Just an arbitrary versioning scheme.

    mapping (address => bool) frozenAccount;
    mapping (address => uint256) lockBox;
    
    /* This generates a public event on the blockchain that will notify clients */
    event FrozenFunds(address _target, bool _frozen);
    event QueryFunds(address _target, uint256 value, uint256 _locked, bool _frozen);
    
    function SAMPLEToken(
        address _initialOwner,
        uint256 _initialAmount,
        string _tokenName,
        uint8 _decimalUnits,
        string _tokenSymbol
        ) TokenERC20(_initialOwner, _initialAmount, _tokenName, _tokenSymbol, _decimalUnits)
          owned(_initialOwner) {
        balanceOf[_initialOwner] = _initialAmount;               // Give the creator all initial tokens
        totalSupply = _initialAmount;                        // Update total supply
        name = _tokenName;                                   // Set the name for display purposes
        decimals = _decimalUnits;                            // Amount of decimals for display purposes
        symbol = _tokenSymbol;                               // Set the symbol for display purposes
    }

    /* Approves and then calls the receiving contract */
    function approveAndCall(address _spender, uint256 _value, bytes _extraData) returns (bool success) {
        allowance[msg.sender][_spender] = _value;
        Approval(msg.sender, _spender, _value);

        //call the receiveApproval function on the contract you want to be notified. This crafts the function signature manually so one doesn't have to include a contract in here just for this.
        //receiveApproval(address _from, uint256 _value, address _tokenContract, bytes _extraData)
        //it is assumed when one does this that the call *should* succeed, otherwise one would use vanilla approve instead.
        require(_spender.call(bytes4(bytes32(sha3("receiveApproval(address,uint256,address,bytes)"))), msg.sender, _value, this, _extraData));
        return true;
    }


    /// @notice Create `mintedAmount` tokens and send it to `target`
    /// @param target Address to receive the tokens
    /// @param mintedAmount the amount of tokens it will receive
    function mintToken(address target, uint256 mintedAmount) onlyOwner public {
        balanceOf[target] += mintedAmount;
        totalSupply += mintedAmount;
        Transfer(0, this, mintedAmount);
        Transfer(this, target, mintedAmount);
    }

    /// @notice `freeze? Prevent | Allow` `target` from sending & receiving tokens
    /// @param target Address to be frozen
    /// @param freeze either to freeze it or not
    function freezeAccount(address target, bool freeze) onlyOwner public {
        frozenAccount[target] = freeze;
        FrozenFunds(target, freeze);
    }   /* added return value */

    function _transferFrom(address _from, address _to, uint _value) 
        returns (bool)  {
        if (!((_to != 0x0) &&
             (balanceOf[_from] > _value)  &&
             (balanceOf[_to] + _value > balanceOf[_to])  &&
             (!frozenAccount[_from])  &&
             (!frozenAccount[_to]))) {
            return false;
        }
        balanceOf[_from] -= _value;
        balanceOf[_to] += _value;
        Transfer(_from, _to, _value);
        return true;
    }


    function lockToken(address target, uint256 lockedAmount) public 
            returns(bool) {

        //1. enough funds 2.lockbox empty 3.account is not frozen
        if ((lockBox[target] != 0) ||
            (balanceOf[target] < lockedAmount)  ||
            (frozenAccount[target])) {
            return false;
        }
        
        balanceOf[target] -= lockedAmount;
        lockBox[target] += lockedAmount;       
    
        return (true);
    }

    function unLockToken(address target) public 
            returns(bool) {

        //1. account is not frozen
        if (frozenAccount[target]) {
            return false;
        }
        var lockedAmount = lockBox[target];
        balanceOf[target] += lockedAmount;
        lockBox[target] = 0;       
    
        return (true);
    }

    function getAccountBalanceOf(address target) public view 
            onlyOwner returns(uint256) {
        Balance(target, balanceOf[target], "query account");
        return (balanceOf[target]);
    }

    function getLockBoxBalanceOf(address target) public view
            onlyOwner returns(uint256) {
        return (lockBox[target]);
    }


    function getMyAccountBalanceOf() public view
            returns(uint256) {
        Balance(msg.sender, balanceOf[msg.sender], "query account");
        return (balanceOf[msg.sender]);
    }

    function getMyLockBoxBalanceOf() public view
            returns(uint256) {
        return (lockBox[msg.sender]);
    }

}

