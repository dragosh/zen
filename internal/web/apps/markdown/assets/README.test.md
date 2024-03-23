---
id: 12
title: Title
layout: page
---

# How to use xxx

# Cat videos

::youtube[Video of a cat in a box]{#PeLgFCbVRXM}

:::{note}
if you chose xxx, you should also use yyy somewhere…
:::

# Head

Lift($$L$$) can be determined by Lift Coefficient ($$C_L$$) like the following
equation.

```math
L = \frac{1}{2} \rho v^2 S C_L
```
## heading

```js {1,3-4} showLineNumbers
function fancyAlert(arg) {
  if (arg) {
    $.facebox({ div: '#foo' })
  }
}
```


:::main{#readme}

Lorem:br
ipsum.

::hr{.red}

A :i[lovely] language know as :abbr[HTML]{title="HyperText Markup Language"}.

:::


:::note{.warning}
if you chose xxx, you should also use yyy somewhere…
:::

```python
print('this is python')
```

```{important}
Here is a note!
```

```{tip}
tip someple
```

````{important}
```{note}
Here's my `important`, highly nested note! 🪆
```
````

```{list-table} This is a nice table!
:header-rows: 1
:name: example-table

* - Training
  - Validation
* - 0
  - 5
* - 13720
  - 2744
```

| foo | bar |
| --- | --- |
| baz | bim |

~~Hi~~ Hello, ~there~ world!

- [x] foo
  - [ ] bar
  - [x] ba
- [ ] bim

Visit www.commonmark.org/help for more information.

Joy :joy:

```js
const a = () => {
  return 2;
};
```

Apple
: Pomaceous fruit of plants of the genus Malus in
the family Rosaceae.

Orange
: The fruit of an evergreen tree of the genus Citrus.

That's some text with a footnote.[^1]

# Vulputate ut pharetra sit amet.

Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Phasellus vestibulum lorem sed risus ultricies. Adipiscing vitae proin sagittis nisl rhoncus mattis rhoncus urna. Purus in massa tempor nec feugiat nisl pretium.

Enim lobortis scelerisque fermentum dui faucibus. Commodo viverra maecenas accumsan lacus. Ac ut consequat semper viverra nam libero justo laoreet sit. Magna etiam tempor orci eu lobortis elementum. Eget sit amet tellus cras adipiscing enim. **Sagittis nisl rhoncus mattis rhoncus urna neque viverra justo nec**.

## Quis varius quam quisque id diam vel.

Sit amet nisl suscipit adipiscing bibendum est ultricies integer quis. Faucibus scelerisque eleifend donec pretium vulputate sapien nec sagittis aliquam. Ac turpis egestas maecenas pharetra convallis. Interdum velit laoreet id donec ultrices tincidunt arcu. Et magnis dis parturient montes nascetur ridiculus mus mauris. Lectus mauris ultrices eros in cursus.

> Orci a scelerisque purus semper eget duis.

Ac auctor augue mauris augue neque gravida in fermentum et. Et pharetra pharetra massa massa ultricies mi quis hendrerit. Vulputate dignissim suspendisse in est ante in nibh mauris. Varius sit amet mattis vulputate enim nulla. **Feugiat nisl pretium fusce id velit ut tortor pretium.** Non tellus orci ac auctor augue mauris. Bibendum neque egestas congue quisque egestas diam in arcu cursus.

### Duis at consectetur lorem donec massa sapien faucibus et.

A diam maecenas sed enim. Feugiat pretium nibh ipsum consequat. Tellus rutrum tellus pellentesque eu tincidunt tortor aliquam. Massa id neque aliquam vestibulum morbi blandit cursus risus. Est ultricies integer quis auctor elit sed vulputate mi sit. Adipiscing bibendum est ultricies integer.

1. At tellus at urna condimentum mattis pellentesque id nibh tortor.
2. Eget dolor morbi non arcu risus quis varius quam quisque.
3. Nibh tellus molestie nunc non.
4. Sed faucibus turpis in eu mi bibendum neque.
5. Posuere sollicitudin aliquam ultrices sagittis orci a scelerisque.

[^1]: And that's the footnote.


# Hello

```js
const a = "test";
```

{{ title }}

![](./assets/logo.png)

table render

| Item         | Price | # In stock |
| ------------ | :---: | ---------: |
| Juicy Apples | 1.99  |        739 |
| Bananas      | 1.89  |          6 |

```mermaid
graph TD
    A[Enter Chart Definition] --> B(Preview)
    B --> C{decide}
    C --> D[Keep]
    C --> E[Edit Definition]
    E --> B
    D --> F[Save Image and Code]
    F --> B
```


```mermaid
C4Context
    title System Context diagram for Internet Banking System
    Enterprise_Boundary(b0, "BankBoundary0") {
    Person(customerA, "Banking Customer A", "A customer of the bank, with personal bank accounts.")
    Person(customerB, "Banking Customer B")
    Person_Ext(customerC, "Banking Customer C", "desc")

    Person(customerD, "Banking Customer D", "A customer of the bank, <br/> with personal bank accounts.")

    System(SystemAA, "Internet Banking System", "Allows customers to view information about their bank accounts, and make payments.")

    Enterprise_Boundary(b1, "BankBoundary") {

        SystemDb_Ext(SystemE, "Mainframe Banking System", "Stores all of the core banking information about customers, accounts, transactions, etc.")

        System_Boundary(b2, "BankBoundary2") {
        System(SystemA, "Banking System A")
        System(SystemB, "Banking System B", "A system of the bank, with personal bank accounts. next line.")
        }

        System_Ext(SystemC, "E-mail system", "The internal Microsoft Exchange e-mail system.")
        SystemDb(SystemD, "Banking System D Database", "A system of the bank, with personal bank accounts.")

        Boundary(b3, "BankBoundary3", "boundary") {
        SystemQueue(SystemF, "Banking System F Queue", "A system of the bank.")
        SystemQueue_Ext(SystemG, "Banking System G Queue", "A system of the bank, with personal bank accounts.")
        }
    }
    }

    BiRel(customerA, SystemAA, "Uses")
    BiRel(SystemAA, SystemE, "Uses")
    Rel(SystemAA, SystemC, "Sends e-mails", "SMTP")
    Rel(SystemC, customerA, "Sends e-mails to")

    UpdateElementStyle(customerA, $fontColor="red", $bgColor="grey", $borderColor="red")
    UpdateRelStyle(customerA, SystemAA, $textColor="blue", $lineColor="blue", $offsetX="5")
    UpdateRelStyle(SystemAA, SystemE, $textColor="blue", $lineColor="blue", $offsetY="-10")
    UpdateRelStyle(SystemAA, SystemC, $textColor="blue", $lineColor="blue", $offsetY="-40", $offsetX="-50")
    UpdateRelStyle(SystemC, customerA, $textColor="red", $lineColor="red", $offsetX="-50", $offsetY="20")

    UpdateLayoutConfig($c4ShapeInRow="3", $c4BoundaryInRow="1")


```