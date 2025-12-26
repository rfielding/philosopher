# Philosopher Usage Examples

This document provides examples of how to use Philosopher to convert natural language requirements into CSP and CTL specifications.

## Example 1: Simple Vending Machine

### Natural Language Description
```
A vending machine accepts coins. After receiving a coin, 
the user can choose either coffee or tea. After dispensing 
the drink, the machine returns to its initial state.
```

### Generated CSP
```csp
VM = coin → CHOICE

CHOICE = coffee → DISPENSE 
       □ tea → DISPENSE

DISPENSE = dispense → VM
```

### Generated CTL Properties
```ctl
# Safety: Can't dispense without payment
AG(¬payment → ¬dispense)

# Liveness: After payment, drink is eventually dispensed
AG(payment → AF(dispense))

# No deadlock
AG(EX(true))
```

### Generated Diagram
```
[Initial] --coin--> [Choice]
[Choice] --coffee--> [Dispense]
[Choice] --tea--> [Dispense]
[Dispense] --dispense--> [Initial]
```

## Example 2: Mutual Exclusion

### Natural Language Description
```
Two processes need to access a shared resource. Only one 
process can access the resource at a time. A process must 
request access, wait to acquire it, use the resource, and 
then release it.
```

### Generated CSP
```csp
PROCESS(i) = request.i → acquire.i → 
             critical.i → release.i → PROCESS(i)

RESOURCE = acquire.1 → release.1 → RESOURCE
         □ acquire.2 → release.2 → RESOURCE

SYSTEM = (PROCESS(1) || PROCESS(2)) || RESOURCE
```

### Generated CTL Properties
```ctl
# Mutual exclusion: Never both in critical section
AG(¬(critical.1 ∧ critical.2))

# No starvation: Every request eventually succeeds
AG(request.1 → AF(acquire.1))
AG(request.2 → AF(acquire.2))

# Progress: Critical section is eventually exited
AG(critical.1 → AF(¬critical.1))
AG(critical.2 → AF(¬critical.2))
```

## Example 3: Producer-Consumer with Buffer

### Natural Language Description
```
A producer generates items and places them in a bounded buffer.
A consumer takes items from the buffer. The buffer has capacity 
for 3 items. The producer waits when the buffer is full. The 
consumer waits when the buffer is empty.
```

### Generated CSP
```csp
BUFFER(n) = 
  n < 3 & produce?item → BUFFER(n+1)
  □
  n > 0 & consume!item → BUFFER(n-1)

PRODUCER = generate → produce!item → PRODUCER

CONSUMER = consume?item → use → CONSUMER

SYSTEM = PRODUCER || BUFFER(0) || CONSUMER
```

### Generated CTL Properties
```ctl
# Safety: Buffer never exceeds capacity
AG(buffer_size ≤ 3)

# Safety: Buffer never goes negative
AG(buffer_size ≥ 0)

# Liveness: Items are eventually consumed
AG(produced → AF(consumed))

# No permanent blocking
AG(EF(produce_possible))
AG(EF(consume_possible))
```

## Example 4: Traffic Light System

### Natural Language Description
```
A traffic light cycles through green, yellow, and red. Green 
lasts longest, yellow is brief, and red lasts a medium time.
The light must go through yellow before changing from green 
to red. The light can go directly from red to green.
```

### Generated CSP
```csp
LIGHT = green → GREEN
GREEN = timeout → yellow → YELLOW
YELLOW = timeout → red → RED
RED = timeout → green → GREEN
```

### Generated CTL Properties
```ctl
# Safety: Green never directly followed by red
AG(green → AX(¬red))

# Safety: Yellow always followed by red
AG(yellow → AX(red))

# Liveness: Each color occurs infinitely often
AG(AF(green))
AG(AF(yellow))
AG(AF(red))

# Sequence property
AG(green → AF(yellow ∧ AF(red)))
```

## Example 5: Request-Response Protocol

### Natural Language Description
```
A client sends requests to a server. The server must acknowledge 
each request and eventually send a response. The client waits for 
a response before sending another request. Requests can fail, and 
failed requests can be retried.
```

### Generated CSP
```csp
CLIENT = send_request → WAIT
WAIT = receive_ack → RESPONSE
RESPONSE = receive_response → CLIENT
         □ receive_failure → CLIENT

SERVER = receive_request → send_ack → PROCESS
PROCESS = send_response → SERVER
        □ send_failure → SERVER

SYSTEM = CLIENT || SERVER
```

### Generated CTL Properties
```ctl
# Every request is acknowledged
AG(send_request → AF(receive_ack))

# Every request eventually gets a response or failure
AG(send_request → AF(receive_response ∨ receive_failure))

# No simultaneous multiple requests from client
AG(waiting → ¬send_request)

# Server always ready to accept new request after completing previous
AG(send_response → AX(EX(receive_request)))
```

## Example 6: Dining Philosophers

### Natural Language Description
```
Five philosophers sit at a round table with five forks. Each 
philosopher needs two forks to eat (one on their left and one 
on their right). Philosophers alternate between thinking and 
eating. They must avoid deadlock and starvation.
```

### Generated CSP
```csp
PHILOSOPHER(i) = think.i → 
                 pickup.i → pickup.((i+1) mod 5) → 
                 eat.i → 
                 putdown.i → putdown.((i+1) mod 5) → 
                 PHILOSOPHER(i)

FORK(i) = pickup.i → putdown.i → FORK(i)
        □ pickup.((i-1) mod 5) → putdown.((i-1) mod 5) → FORK(i)

SYSTEM = (|| i : {0..4} @ PHILOSOPHER(i)) 
      || (|| i : {0..4} @ FORK(i))
```

### Generated CTL Properties
```ctl
# Safety: No two adjacent philosophers eat simultaneously
AG(∀i: eating.i → ¬eating.((i+1) mod 5))

# No deadlock: Always possible to pick up a fork
AG(EX(∃i: pickup.i))

# Liveness: Every philosopher eventually eats
AG(∀i: thinking.i → AF(eating.i))

# Fairness: No philosopher starves
AG(∀i: AF(eating.i))
```

## Example 7: Elevator Control System

### Natural Language Description
```
An elevator serves three floors. People can request the elevator 
from any floor. The elevator moves to requested floors in order. 
The doors open at each stop, wait for passengers, then close 
before moving.
```

### Generated CSP
```csp
ELEVATOR(floor) = 
  request?n → MOVING(floor, n)

MOVING(current, target) =
  current = target & arrive.target → DOORS_OPEN(current)
  □
  current < target & move_up → MOVING(current+1, target)
  □
  current > target & move_down → MOVING(current-1, target)

DOORS_OPEN(floor) = 
  open_doors.floor → wait → close_doors.floor → ELEVATOR(floor)

SYSTEM = ELEVATOR(0)
```

### Generated CTL Properties
```ctl
# Safety: Doors only open when elevator is stopped
AG(doors_open → ¬moving)

# Safety: Elevator doesn't exceed floor bounds
AG(0 ≤ current_floor ≤ 2)

# Liveness: Requests are eventually served
AG(request.n → AF(arrive.n))

# Progress: Elevator doesn't stay in one place forever
AG(EF(moving))
```

## Tips for Writing Requirements

### Be Specific
❌ Bad: "The system should work correctly"
✓ Good: "After a user logs in, they must be redirected to the dashboard"

### Use Clear Sequences
❌ Bad: "Things happen and then other things"
✓ Good: "First, the user clicks submit. Then, the system validates. Finally, a confirmation appears"

### Specify Constraints
❌ Bad: "Multiple users can access the system"
✓ Good: "Multiple users can read simultaneously, but only one user can write at a time"

### Include Error Cases
❌ Bad: "The system processes requests"
✓ Good: "The system processes valid requests and rejects invalid ones with an error message"

### Define Termination
❌ Bad: "The process runs"
✓ Good: "The process runs until completion or until cancelled by the user"

## Common Patterns

### State Machines
```
Initial state → Event → Next state → ...
```

### Request-Response
```
Request → Process → Response
```

### Resource Allocation
```
Request → Wait → Acquire → Use → Release
```

### Producer-Consumer
```
Produce → Buffer → Consume
```

### Synchronization
```
Process A and Process B must coordinate on event X
```

## Verification Results Format

### Success
```
✓ Property verified: AG(safe)
  All 1,247 states checked
  No violations found
```

### Failure with Counterexample
```
✗ Property violated: AF(complete)
  Counterexample trace:
    State 0: init
    State 1: start
    State 2: waiting
    State 3: waiting (loop detected)
  Issue: System can wait forever
```

### Suggestion
```
💡 Suggestion: Add a timeout to prevent infinite waiting
```

## Next Steps

After seeing these examples, you can:
1. Start with a simple system description
2. Review the generated CSP
3. Check the CTL properties
4. Visualize the state diagram
5. Run model checking
6. Refine based on results

For more information, see:
- [CONTEXT.md](CONTEXT.md) - Project overview
- [CSP_REFERENCE.md](CSP_REFERENCE.md) - CSP language details
- [CTL_REFERENCE.md](CTL_REFERENCE.md) - CTL logic details
- [DEVELOPMENT.md](DEVELOPMENT.md) - Development information
