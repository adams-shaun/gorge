"""The TestGNNParity fixture's network: a tiny GraphNet with every weight randomised.

Run from draft-zero's gorge/ directory with it on PYTHONPATH (it imports dzg), then export and dump
(see gnn_parity_test.go):

    PYTHONPATH=. python <gorge>/bots/gonet/testdata/gnn/mktiny.py /tmp/tiny.pt

Not trained: a fresh GraphNet zero-initialises its policy and value heads' last layers, so this perturbs
every parameter (LayerNorm gains around 1) to make each path of the forward carry signal.
"""
import sys

import torch
from torch import nn

from dzg import models as M

torch.manual_seed(20261023)
m = M.build("gnn", {"d": 16, "heads": 2, "ff": 32, "d_emb": 4, "head_hidden": 16, "passes": 2,
                    "global_layers": 2, "policy_layers": 2, "value_layers": 1})
with torch.no_grad():
    for mod_name, mod in m.named_modules():
        for name, p in mod.named_parameters(recurse=False):
            if isinstance(mod, nn.LayerNorm) and name == "weight":
                p.copy_(1 + 0.3 * torch.randn_like(p))
            elif mod_name == "table":
                p.copy_(0.5 * torch.randn_like(p))
            else:
                p.add_(0.25 * torch.randn_like(p))
M.save(m, sys.argv[1], meta={"note": "gorge bots/gonet TestGNNParity fixture: random weights, not trained"})
print(m.parameter_counts())
