namespace FileSorter.Core.Models;

public class SortingProfile
{
    public SortingCriteria PrimaryCriteria { get; set; } = SortingCriteria.Extension;
    
    public SortingCriteria SecondaryCriteria { get; set; } = SortingCriteria.None;
    
    public DateGroupingOption DatePrimaryGroupingOption { get; set; } = DateGroupingOption.None;
    
    public DateGroupingOption DateSecondaryGroupingOption { get; set; } = DateGroupingOption.None;
    
    public FileOperationType FileOperationType { get; set; } = FileOperationType.Move;
}

// TODO: TBD
public class ConflictHandling
{
        
}
    
public enum SortingCriteria
{
    None,
    CreationDate,
    ModificationDate,
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
    YearMonthDay
}

 