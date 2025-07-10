import torch
from torchvision import models

def create_model(num_classes, model_name='resnet50', pretrained=True, freeze_backbone=True):
    """
    Creates ResNet model

    Args:
        num_classes: number of classes
        model_name: model type ('resnet18', 'resnet34', 'resnet50', 'resnet101', 'resnet152')
        pretrained: true or false if use pretrained model (by default True)
        freeze_backbone: freeze backbone for transfer learning (by default True)
    """
    
    model = None

    if model_name == 'resnet18':
        model = models.resnet18(pretrained=pretrained)
    elif model_name == 'resnet34':
        model = models.resnet34(pretrained=pretrained)
    elif model_name == 'resnet50':
        model = models.resnet50(pretrained=pretrained)
    elif model_name == 'resnet101':
        model = models.resnet101(pretrained=pretrained)
    elif model_name == 'resnet152':
        model = models.resnet152(pretrained=pretrained)
    else:
        raise ValueError(f"Unavalible model: {model_name}")
    
    if freeze_backbone and pretrained:
        for param in model.parameters():
            param.requires_grad = False
    
    # replace last layer
    num_features = model.fc.in_features
    model.fc = torch.nn.Linear(num_features, num_classes)
    
    return model
