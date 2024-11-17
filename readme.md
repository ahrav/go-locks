# go-locks

go-locks is a Go implementation of various lock algorithms from the [libslock](https://github.com/tudordavid/libslock) C library.
These locks were featured in the research paper
"Everything You Always Wanted to Know About Synchronization but Were Afraid to Ask" by Tudor David, Rachid Guerraoui, and Vasileios Trigonakis[1].

## Overview

This library provides Go implementations of several lock algorithms, including:

- Ticket Lock
- MCS Lock
- CLH Lock
- (Add other locks you've implemented)

The goal of this project is to explore and learn about different synchronization techniques in Go,
based on the algorithms presented in the libslock library.

DO NOT USE THIS LIBRARY anywhere near production code. It is purely for educational purposes.