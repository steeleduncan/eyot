# Structs

As with other languages structs allow you to gather a set of variables, and functions acting on those variables, into a single place.

TODO this uses `sqrt`, which would presumably be stdlib when that is a thing

```
struct Vector {
    x, y f64 
    
    // a GPU capable function
    fn length() f4 {
        return sqrt(self.x * self.x + self.y * self.y)
    }

    // a CPU only function
    cpu fn log() {
        print_ln("(", self.x, ", ", self.y, ")")
    }
}	

cpu fn main() {
    let v = Vector { x: 3, y: 4 }
    v.log()
    print_ln("l = ", v.length())
}

```

Functions on a struct can access struct variables through the implicit `self` parameter, but otherwise they follow the usual rules for functions, including those for location independent code

It is only relevant to know when considering workers below, but the `.` operator on a struct returns a partially applied function.
Continuing the example above, you can write

```
let f = v.length
print_ln(f())

```

