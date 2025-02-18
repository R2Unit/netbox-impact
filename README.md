# Netbox Impact - Midleware

**Formula:**

A forum module is first calculated based on the weight given by the user and the weight per device, which are calculated in the formula below

- $D$ is the count of devices
- $C$ is the count of circuits
- $I$ is the count of interfaces
- $M$ is the impact type multiplier
- $R$  is the impact type multiplier (e.g., 1.0, 1.5, or 2.0)

To account for redudance in the circuits, there is a factor $Ri$ for every circuit $i$ for a circuit with both terminations connected on the same node.  

$$
Impact=M×(5D+i=1∑C​(3×Ri​)+I)
$$

**Computed formula**

> `\text{Impact} = M \times \Bigl(5 \times \text{# Devices} + \sum_{i=1}^{C}\bigl(3 \times R_i\bigr) + 1 \times \text{# Interfaces}\Bigr)`
> 

### Final Impact Formula

After checking the weights and the formula we get a final impact formula which is as follows.

$$
Impact=M×(5D+Total Circuit Impact+I)
$$