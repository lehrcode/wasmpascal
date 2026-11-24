unit Werkzeug;

interface

const
  Strich = '-';

type
  TPunkt = record
    x, y: Integer
  end;

function Rahmen(const s: string): string;

implementation

uses Zaehler;

function Rahmen;
begin
  Rahmen := Strich + s + Strich + ' (' + Zaehler.AlsText + ')'
end;

begin
  WriteLn('Init Werkzeug')
end.
