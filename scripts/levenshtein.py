# coding: utf-8
#

import numpy as np


def match_string(a, b):
    a = ' ' + a
    b = ' ' + b
    array = np.zeros((len(a), len(b)), dtype=np.int)
    steps = np.zeros((len(a), len(b)), dtype=np.int)
    array[0] = np.arange(len(b))
    array[:, 0] = np.arange(len(a))
    steps[0] = 1

    for i in range(1, len(a)):
        for j in range(1, len(b)):
            sub_cost = 0 if a[i] == b[j] else 2
            minval = array[i-1, j-1]+sub_cost # substitution
            steps[i, j] = 2
            
            del_cost = 1 if a[i] != ' ' else 0
            ins_cost = 1 if b[j] != ' ' else 0
            # delete, insertion
            for step, val in enumerate((array[i-1, j]+del_cost, array[i, j-1]+ins_cost)):
                if minval > val:
                    steps[i, j] = step
                    minval = val
            array[i, j] = minval
    
    # print(array)
    # print(steps)
    return array, steps


def backward(steps, a, b):
    assert len(steps) == len(a)+1
    assert len(steps[0]) == len(b)+1

    x = len(a)
    y = len(b)

    ss = []
    while x > 0 or y > 0:
        step = steps[x, y]
        # print(x, y)
        if step == 0:
            x, y = x-1, y
            ss.append(['d', a[x], ' '])
        elif step == 1:
            x, y = x, y-1
            ss.append(['i', ' ', b[y]])
        elif step == 2:
            x, y = x-1, y-1
            if a[x] == b[y]:
                ss.append(['=', a[x], b[y]])
            else:
                ss.append(['r', a[x], b[y]])
    # for i in range(len(ss)):
        # print()
    for line in map(list, zip(*ss)):
        print(''.join(reversed(line)))


def main():
    a = "sitting"
    b = "kitten"
    # a, b = "Monday", "Hello world"
    array, steps = match_string(a, b)
    backward(steps, a, b)


def match_distance(a, b):
    array, steps = match_string(a, b)
    if False:
        backward(steps, a, b)
    return int(array[-1, -1])


if __name__ == '__main__':
    print(match_distance('sitting', 'kitten'))