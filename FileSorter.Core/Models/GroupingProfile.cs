﻿namespace FileSorter.Core.Models;

public class GroupingProfile
{
    public GroupingCriteria PrimaryCriteria { get; set; } = GroupingCriteria.Extension;
    
    public GroupingCriteria SecondaryCriteria { get; set; } = GroupingCriteria.None;
    
    public DateGroupingOption DatePrimaryGroupingOption { get; set; } = DateGroupingOption.None;
    
    public DateGroupingOption DateSecondaryGroupingOption { get; set; } = DateGroupingOption.None;
    
    public FileOperationType FileOperationType { get; set; } = FileOperationType.Move;
    
    public ConflictHandling ConflictHandling { get; set; } = new ConflictHandling
    {
        Strategy = ConflictResolutionStrategy.Ask
    };
}
    
public enum GroupingCriteria
{
    None,
    CreationDate,
    ModificationDate,
    OldestDate,
    Extension,
    FileCategory
}
    
public enum FileOperationType
{
    Move,
    Copy
}
public enum DateGroupingOption
    
{
    None,
    Year,
    Month,
    YearMonth,
}

 